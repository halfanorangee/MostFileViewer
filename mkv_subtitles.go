package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// EmbeddedSubtitleTrack 描述 Matroska/WebM 容器中的一条可提取文本字幕轨。
// Number 是容器内的 TrackNumber，供 ReadEmbeddedSubtitle 精确定位。
type EmbeddedSubtitleTrack struct {
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Language string `json:"language"`
	Codec    string `json:"codec"`
	Label    string `json:"label"`
}

const (
	idEBML             = 0x1A45DFA3
	idEBMLDocType      = 0x4282
	idSegment          = 0x18538067
	idSeekHead         = 0x114D9B74
	idSeek             = 0x4DBB
	idSeekID           = 0x53AB
	idSeekPosition     = 0x53AC
	idInfo             = 0x1549A966
	idTimestampScale   = 0x2AD7B1
	idTracks           = 0x1654AE6B
	idTrackEntry       = 0xAE
	idTrackNumber      = 0xD7
	idTrackType        = 0x83
	idName             = 0x536E
	idCodecID          = 0x86
	idLanguage         = 0x22B59C
	idLanguageIETF     = 0x22B59D
	idDefaultDuration  = 0x23E383
	idContentEncodings = 0x6D80
	idCluster          = 0x1F43B675
	idTimestamp        = 0xE7
	idSimpleBlock      = 0xA3
	idBlockGroup       = 0xA0
	idBlock            = 0xA1
	idBlockDuration    = 0x9B

	mkvTrackTypeSubtitle   = 0x11
	mkvDefaultTimestampNs  = 1_000_000
	mkvDefaultCueDuration  = 2 * time.Second
	mkvMaxInMemoryElement  = 4 << 20
	mkvMaxClusterBuffer    = 16 << 20
	mkvMaxExtractedVTTSize = maxSubtitleFileSize
)

var (
	errNotMatroska     = errors.New("不是 Matroska/WebM 容器")
	errElementTooLarge = errors.New("mkv: 元素过大")
)

const mkvSubtitleCacheLimit = 64 << 20

type mkvSubtitleCacheEntry struct {
	size     int64
	modified int64
	tracks   map[int]string
	bytes    int64
	lastUsed time.Time
}

var mkvSubtitleCache = struct {
	sync.Mutex
	items map[string]*mkvSubtitleCacheEntry
	bytes int64
}{items: make(map[string]*mkvSubtitleCacheEntry)}

type mkvSubTrack struct {
	number          uint64
	name            string
	language        string
	codec           string
	defaultDuration time.Duration
	extractable     bool
}

type mkvCue struct {
	start time.Duration
	end   time.Duration
	text  string
}

type mkvReader struct {
	f   *os.File
	br  *bufio.Reader
	pos int64
}

func newMKVReader(f *os.File) *mkvReader {
	return &mkvReader{f: f, br: bufio.NewReaderSize(f, 64<<10), pos: 0}
}

func (r *mkvReader) seek(pos int64) error {
	if pos == r.pos {
		return nil
	}
	if _, err := r.f.Seek(pos, io.SeekStart); err != nil {
		return err
	}
	r.br.Reset(r.f)
	r.pos = pos
	return nil
}

func (r *mkvReader) readByte() (byte, error) {
	b, err := r.br.ReadByte()
	if err != nil {
		return 0, err
	}
	r.pos++
	return b, nil
}

func (r *mkvReader) readFull(n int) ([]byte, error) {
	if n <= 0 {
		return []byte{}, nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r.br, buf); err != nil {
		return nil, err
	}
	r.pos += int64(n)
	return buf, nil
}

func (r *mkvReader) skip(n uint64) error {
	if n == 0 {
		return nil
	}
	if n > uint64(math.MaxInt64-r.pos) {
		return errors.New("mkv: 跳过范围溢出")
	}
	target := r.pos + int64(n)
	buffered := r.br.Buffered()
	if n <= uint64(buffered) {
		if _, err := r.br.Discard(int(n)); err != nil {
			return err
		}
		r.pos = target
		return nil
	}
	return r.seek(target)
}

func vintWidth(first byte) (width int, err error) {
	switch {
	case first&0x80 != 0:
		return 1, nil
	case first&0x40 != 0:
		return 2, nil
	case first&0x20 != 0:
		return 3, nil
	case first&0x10 != 0:
		return 4, nil
	case first&0x08 != 0:
		return 5, nil
	case first&0x04 != 0:
		return 6, nil
	case first&0x02 != 0:
		return 7, nil
	case first&0x01 != 0:
		return 8, nil
	default:
		return 0, errors.New("mkv: 无效的 VINT")
	}
}

func vintDataMask(width int) byte {
	return byte(0x80 >> (width - 1))
}

func (r *mkvReader) readElementID() (uint64, error) {
	first, err := r.readByte()
	if err != nil {
		return 0, err
	}
	width, err := vintWidth(first)
	if err != nil {
		return 0, err
	}
	id := uint64(first)
	for i := 1; i < width; i++ {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		id = (id << 8) | uint64(b)
	}
	return id, nil
}

func (r *mkvReader) readElementSize() (size uint64, unknown bool, err error) {
	first, err := r.readByte()
	if err != nil {
		return 0, false, err
	}
	width, err := vintWidth(first)
	if err != nil {
		return 0, false, err
	}
	mask := vintDataMask(width) - 1
	value := uint64(first & mask)
	allOnes := first&mask == mask
	for i := 1; i < width; i++ {
		b, err := r.readByte()
		if err != nil {
			return 0, false, err
		}
		value = (value << 8) | uint64(b)
		if b != 0xFF {
			allOnes = false
		}
	}
	if allOnes {
		return 0, true, nil
	}
	return value, false, nil
}

type mkvElement struct {
	id      uint64
	size    uint64
	unknown bool
	dataEnd int64
}

func (r *mkvReader) nextElement(parentEnd int64) (mkvElement, error) {
	if parentEnd >= 0 && r.pos >= parentEnd {
		return mkvElement{}, io.EOF
	}
	id, err := r.readElementID()
	if err != nil {
		return mkvElement{}, err
	}
	size, unknown, err := r.readElementSize()
	if err != nil {
		return mkvElement{}, err
	}
	el := mkvElement{id: id, size: size, unknown: unknown}
	if unknown {
		el.dataEnd = parentEnd
	} else {
		if size > uint64(math.MaxInt64-r.pos) {
			return mkvElement{}, errors.New("mkv: 元素尺寸溢出")
		}
		el.dataEnd = r.pos + int64(size)
		if parentEnd >= 0 && el.dataEnd > parentEnd {
			el.dataEnd = parentEnd
		}
	}
	return el, nil
}

func (r *mkvReader) readPayload(el mkvElement) ([]byte, error) {
	if el.unknown {
		return nil, errors.New("mkv: 无法读取未知长度的元素")
	}
	size := el.dataEnd - r.pos
	if size < 0 {
		return nil, errors.New("mkv: 元素尺寸为负")
	}
	if size > mkvMaxInMemoryElement {
		return nil, errElementTooLarge
	}
	return r.readFull(int(size))
}

func (r *mkvReader) skipElement(el mkvElement) error {
	if el.unknown {
		return errors.New("mkv: 无法跳过未知长度的元素")
	}
	if el.dataEnd < r.pos {
		return errors.New("mkv: 元素终点早于当前位置")
	}
	return r.seek(el.dataEnd)
}

func parseUintBytes(data []byte) uint64 {
	var v uint64
	for _, b := range data {
		v = (v << 8) | uint64(b)
	}
	return v
}

func parseEBMLString(data []byte) string {
	return strings.TrimRight(string(data), "\x00")
}

func isExtractableSubtitleCodec(codec string) bool {
	switch codec {
	case "S_TEXT/UTF8", "S_TEXT/ASCII", "S_TEXT/WEBVTT", "S_TEXT/ASS", "S_TEXT/SSA":
		return true
	default:
		return false
	}
}

func listMatroskaSubtitleTracks(path string) ([]EmbeddedSubtitleTrack, error) {
	session, err := openMatroska(path)
	if err != nil {
		return nil, err
	}
	defer session.close()
	return session.exportedTracks(), nil
}

func extractMatroskaSubtitle(path string, trackNumber int) (string, error) {
	if trackNumber <= 0 {
		return "", errors.New("无效的字幕轨道")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	modified := info.ModTime().UnixNano()
	if text, ok := getCachedSubtitle(path, info.Size(), modified, trackNumber); ok {
		return text, nil
	}
	session, err := openMatroska(path)
	if err != nil {
		return "", err
	}
	defer session.close()
	tracks, err := session.extractAll()
	if err != nil {
		return "", err
	}
	cacheSubtitles(path, info.Size(), modified, tracks)
	text, ok := tracks[trackNumber]
	if !ok {
		return "", errors.New("未找到可提取的内嵌字幕轨道")
	}
	return text, nil
}

func getCachedSubtitle(path string, size, modified int64, trackNumber int) (string, bool) {
	mkvSubtitleCache.Lock()
	defer mkvSubtitleCache.Unlock()
	entry, ok := mkvSubtitleCache.items[path]
	if !ok {
		return "", false
	}
	if entry.size != size || entry.modified != modified {
		deleteCachedSubtitleLocked(path)
		return "", false
	}
	text, ok := entry.tracks[trackNumber]
	if ok {
		entry.lastUsed = time.Now()
	}
	return text, ok
}

func cacheSubtitles(path string, size, modified int64, tracks map[int]string) {
	if len(tracks) == 0 {
		return
	}
	entry := &mkvSubtitleCacheEntry{
		size:     size,
		modified: modified,
		tracks:   make(map[int]string, len(tracks)),
		lastUsed: time.Now(),
	}
	for number, text := range tracks {
		entry.tracks[number] = text
		entry.bytes += int64(len(text))
	}
	if entry.bytes > mkvSubtitleCacheLimit {
		return
	}

	mkvSubtitleCache.Lock()
	defer mkvSubtitleCache.Unlock()
	if old := mkvSubtitleCache.items[path]; old != nil {
		mkvSubtitleCache.bytes -= old.bytes
	}
	mkvSubtitleCache.items[path] = entry
	mkvSubtitleCache.bytes += entry.bytes
	for mkvSubtitleCache.bytes > mkvSubtitleCacheLimit {
		var oldestPath string
		var oldest time.Time
		for candidatePath, candidate := range mkvSubtitleCache.items {
			if oldestPath == "" || candidate.lastUsed.Before(oldest) {
				oldestPath = candidatePath
				oldest = candidate.lastUsed
			}
		}
		if oldestPath == "" {
			break
		}
		deleteCachedSubtitleLocked(oldestPath)
	}
}

func deleteCachedSubtitleLocked(path string) {
	if entry := mkvSubtitleCache.items[path]; entry != nil {
		mkvSubtitleCache.bytes -= entry.bytes
		delete(mkvSubtitleCache.items, path)
	}
}

type mkvSession struct {
	r              *mkvReader
	file           *os.File
	fileSize       int64
	segmentStart   int64
	segmentEnd     int64
	timestampScale uint64
	tracks         []mkvSubTrack
	seekPositions  map[uint64]int64
}

func (s *mkvSession) close() {
	if s.file != nil {
		_ = s.file.Close()
	}
}

func openMatroska(path string) (*mkvSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	session := &mkvSession{
		r:              newMKVReader(file),
		file:           file,
		fileSize:       info.Size(),
		timestampScale: mkvDefaultTimestampNs,
		seekPositions:  make(map[uint64]int64),
	}
	if err := session.parseHeaderAndTracks(); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}

func (s *mkvSession) parseHeaderAndTracks() error {
	r := s.r
	el, err := r.nextElement(s.fileSize)
	if err != nil {
		return errNotMatroska
	}
	if el.id != idEBML {
		return errNotMatroska
	}
	docType, err := s.parseDocType(el.dataEnd)
	if err != nil {
		return err
	}
	if docType != "" && docType != "matroska" && docType != "webm" {
		return errNotMatroska
	}

	var seg mkvElement
	for {
		next, err := r.nextElement(s.fileSize)
		if err != nil {
			return fmt.Errorf("mkv: 缺少 Segment: %w", err)
		}
		if next.id == idSegment {
			seg = next
			break
		}
		if err := r.skipElement(next); err != nil {
			return err
		}
	}
	s.segmentStart = r.pos
	s.segmentEnd = seg.dataEnd
	if seg.unknown || s.segmentEnd < 0 || s.segmentEnd > s.fileSize {
		s.segmentEnd = s.fileSize
	}

	if err := s.scanSegmentForTracks(); err != nil {
		return err
	}
	if s.timestampScale == 0 {
		s.timestampScale = mkvDefaultTimestampNs
	}
	return nil
}

func (s *mkvSession) parseDocType(end int64) (string, error) {
	docType := ""
	for {
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if el.id == idEBMLDocType {
			payload, err := s.r.readPayload(el)
			if err != nil {
				return "", err
			}
			docType = strings.ToLower(strings.TrimSpace(parseEBMLString(payload)))
			continue
		}
		if err := s.r.skipElement(el); err != nil {
			return "", err
		}
	}
	if s.r.pos < end {
		if err := s.r.seek(end); err != nil {
			return "", err
		}
	}
	return docType, nil
}

func (s *mkvSession) scanSegmentForTracks() error {
	foundTracks := false
	foundInfo := false
	for {
		el, err := s.r.nextElement(s.segmentEnd)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch el.id {
		case idSeekHead:
			if err := s.parseSeekHead(el.dataEnd); err != nil {
				return err
			}
		case idInfo:
			if err := s.parseInfo(el.dataEnd); err != nil {
				return err
			}
			foundInfo = true
		case idTracks:
			if err := s.parseTracks(el.dataEnd); err != nil {
				return err
			}
			foundTracks = true
		case idCluster:
			if err := s.r.skipElement(el); err != nil {
				return err
			}
			if foundTracks {
				return s.fillMissingFromSeek(foundTracks, foundInfo)
			}
		default:
			if err := s.r.skipElement(el); err != nil {
				return err
			}
		}
		if foundTracks && foundInfo {
			return nil
		}
	}
	return s.fillMissingFromSeek(foundTracks, foundInfo)
}

func (s *mkvSession) fillMissingFromSeek(foundTracks, foundInfo bool) error {
	if !foundTracks {
		if err := s.parseTracksFromSeek(); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	if !foundInfo {
		if err := s.parseInfoFromSeek(); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	return nil
}

func (s *mkvSession) parseSeekHead(end int64) error {
	for {
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if el.id != idSeek {
			if err := s.r.skipElement(el); err != nil {
				return err
			}
			continue
		}
		var seekID uint64
		var seekPos uint64
		innerEnd := el.dataEnd
		for {
			child, err := s.r.nextElement(innerEnd)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			switch child.id {
			case idSeekID, idSeekPosition:
				payload, err := s.r.readPayload(child)
				if err != nil {
					return err
				}
				if child.id == idSeekID {
					seekID = parseUintBytes(payload)
				} else {
					seekPos = parseUintBytes(payload)
				}
			default:
				if err := s.r.skipElement(child); err != nil {
					return err
				}
			}
		}
		if seekID != 0 && seekPos <= uint64(math.MaxInt64-s.segmentStart) {
			s.seekPositions[seekID] = s.segmentStart + int64(seekPos)
		}
	}
	if s.r.pos < end {
		return s.r.seek(end)
	}
	return nil
}

func (s *mkvSession) parseInfo(end int64) error {
	for {
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if el.id == idTimestampScale {
			payload, err := s.r.readPayload(el)
			if err != nil {
				return err
			}
			if scale := parseUintBytes(payload); scale > 0 {
				s.timestampScale = scale
			}
			continue
		}
		if err := s.r.skipElement(el); err != nil {
			return err
		}
	}
	if s.r.pos < end {
		return s.r.seek(end)
	}
	return nil
}

func (s *mkvSession) parseTracks(end int64) error {
	s.tracks = s.tracks[:0]
	for {
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if el.id != idTrackEntry {
			if err := s.r.skipElement(el); err != nil {
				return err
			}
			continue
		}
		track, err := s.parseTrackEntry(el.dataEnd)
		if err != nil {
			return err
		}
		if track.number != 0 && track.codec != "" {
			s.tracks = append(s.tracks, track)
		}
	}
	if s.r.pos < end {
		return s.r.seek(end)
	}
	return nil
}

func (s *mkvSession) parseTrackEntry(end int64) (mkvSubTrack, error) {
	var track mkvSubTrack
	var trackType uint64
	var language, languageIETF string
	encoded := false
	for {
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return mkvSubTrack{}, err
		}
		switch el.id {
		case idTrackNumber, idTrackType, idName, idCodecID, idLanguage, idLanguageIETF, idDefaultDuration:
			payload, err := s.r.readPayload(el)
			if err != nil {
				return mkvSubTrack{}, err
			}
			switch el.id {
			case idTrackNumber:
				track.number = parseUintBytes(payload)
			case idTrackType:
				trackType = parseUintBytes(payload)
			case idName:
				track.name = strings.TrimSpace(payloadToString(payload))
			case idCodecID:
				track.codec = parseEBMLString(payload)
			case idLanguageIETF:
				languageIETF = parseEBMLString(payload)
			case idLanguage:
				language = parseEBMLString(payload)
			case idDefaultDuration:
				if ns := parseUintBytes(payload); ns > 0 && ns <= uint64(math.MaxInt64) {
					track.defaultDuration = time.Duration(ns)
				}
			}
		case idContentEncodings:
			encoded = true
			if err := s.r.skipElement(el); err != nil {
				return mkvSubTrack{}, err
			}
		default:
			if err := s.r.skipElement(el); err != nil {
				return mkvSubTrack{}, err
			}
		}
	}
	if s.r.pos < end {
		if err := s.r.seek(end); err != nil {
			return mkvSubTrack{}, err
		}
	}
	if trackType != mkvTrackTypeSubtitle {
		return mkvSubTrack{}, nil
	}
	if languageIETF != "" {
		track.language = languageIETF
	} else {
		track.language = language
	}
	track.extractable = isExtractableSubtitleCodec(track.codec) && !encoded
	return track, nil
}

func (s *mkvSession) seekToID(id uint64) error {
	pos, ok := s.seekPositions[id]
	if !ok {
		return io.EOF
	}
	if pos < s.segmentStart || pos >= s.segmentEnd {
		return io.EOF
	}
	return s.r.seek(pos)
}

func (s *mkvSession) parseTracksFromSeek() error {
	if len(s.tracks) > 0 {
		return nil
	}
	if err := s.seekToID(idTracks); err != nil {
		return err
	}
	el, err := s.r.nextElement(s.segmentEnd)
	if err != nil {
		return err
	}
	if el.id != idTracks {
		return nil
	}
	return s.parseTracks(el.dataEnd)
}

func (s *mkvSession) parseInfoFromSeek() error {
	if err := s.seekToID(idInfo); err != nil {
		return err
	}
	el, err := s.r.nextElement(s.segmentEnd)
	if err != nil {
		return err
	}
	if el.id != idInfo {
		return nil
	}
	return s.parseInfo(el.dataEnd)
}

func (s *mkvSession) exportedTracks() []EmbeddedSubtitleTrack {
	var out []EmbeddedSubtitleTrack
	index := 0
	for _, track := range s.tracks {
		if !track.extractable || track.number == 0 || track.number > uint64(math.MaxInt) {
			continue
		}
		index++
		out = append(out, EmbeddedSubtitleTrack{
			Number:   int(track.number),
			Name:     track.name,
			Language: track.language,
			Codec:    track.codec,
			Label:    subtitleTrackLabel(track, index),
		})
	}
	if out == nil {
		return []EmbeddedSubtitleTrack{}
	}
	return out
}

func (s *mkvSession) findTrack(number uint64) (mkvSubTrack, bool) {
	for _, track := range s.tracks {
		if track.number == number {
			return track, true
		}
	}
	return mkvSubTrack{}, false
}

func (s *mkvSession) extract(trackNumber uint64) (string, error) {
	track, ok := s.findTrack(trackNumber)
	if !ok || !track.extractable {
		return "", errors.New("未找到可提取的内嵌字幕轨道")
	}
	if s.timestampScale == 0 {
		if err := s.parseInfoFromSeek(); err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if s.timestampScale == 0 {
			s.timestampScale = mkvDefaultTimestampNs
		}
	}
	if err := s.r.seek(s.segmentStart); err != nil {
		return "", err
	}
	var cues []mkvCue
	for {
		el, err := s.r.nextElement(s.segmentEnd)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if el.id != idCluster {
			if err := s.r.skipElement(el); err != nil {
				return "", err
			}
			continue
		}
		if err := s.parseCluster(el, track, &cues); err != nil {
			return "", err
		}
	}
	return cuesToWebVTT(cues)
}

// extractAll 扫描一次 Cluster，同时收集所有可提取的文本字幕轨道。
// 轨道切换由上层缓存处理，避免每次选择都重新遍历整个媒体文件。
func (s *mkvSession) extractAll() (map[int]string, error) {
	if s.timestampScale == 0 {
		if err := s.parseInfoFromSeek(); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if s.timestampScale == 0 {
			s.timestampScale = mkvDefaultTimestampNs
		}
	}
	tracks := make(map[uint64]mkvSubTrack)
	for _, track := range s.tracks {
		if track.extractable && track.number > 0 {
			tracks[track.number] = track
		}
	}
	result := make(map[int]string, len(tracks))
	if len(tracks) == 0 {
		return result, nil
	}
	if err := s.r.seek(s.segmentStart); err != nil {
		return nil, err
	}
	cues := make(map[uint64][]mkvCue, len(tracks))
	for {
		el, err := s.r.nextElement(s.segmentEnd)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if el.id != idCluster {
			if err := s.r.skipElement(el); err != nil {
				return nil, err
			}
			continue
		}
		if err := s.parseClusterAll(el, tracks, cues); err != nil {
			return nil, err
		}
	}
	for number, trackCues := range cues {
		text, err := cuesToWebVTT(trackCues)
		if err == nil {
			if number <= uint64(math.MaxInt) {
				result[int(number)] = text
			}
		}
	}
	return result, nil
}

func (s *mkvSession) parseClusterAll(el mkvElement, tracks map[uint64]mkvSubTrack, cues map[uint64][]mkvCue) error {
	size := el.dataEnd - s.r.pos
	if !el.unknown && size >= 0 && size <= mkvMaxClusterBuffer {
		payload, err := s.r.readFull(int(size))
		if err != nil {
			return err
		}
		return parseClusterBytesAll(payload, s.timestampScale, tracks, cues)
	}
	return s.parseClusterStreamingAll(el.dataEnd, el.unknown, tracks, cues)
}

func parseClusterBytesAll(data []byte, scale uint64, tracks map[uint64]mkvSubTrack, cues map[uint64][]mkvCue) error {
	var clusterTS uint64
	off := 0
	for off < len(data) {
		id, idWidth, err := decodeElementID(data[off:])
		if err != nil {
			return err
		}
		off += idWidth
		size, sizeWidth, unknown, err := decodeElementSize(data[off:])
		if err != nil {
			return err
		}
		off += sizeWidth
		var payload []byte
		if unknown {
			payload = data[off:]
			off = len(data)
		} else {
			if int(size) > len(data)-off {
				return errors.New("mkv: Cluster 子元素越界")
			}
			payload = data[off : off+int(size)]
			off += int(size)
		}
		switch id {
		case idTimestamp:
			clusterTS = parseUintBytes(payload)
		case idSimpleBlock:
			appendSimpleCue(payload, clusterTS, scale, tracks, cues)
		case idBlockGroup:
			appendBlockGroupCue(payload, clusterTS, scale, tracks, cues)
		}
	}
	return nil
}

func (s *mkvSession) parseClusterStreamingAll(end int64, unknown bool, tracks map[uint64]mkvSubTrack, cues map[uint64][]mkvCue) error {
	var clusterTS uint64
	for {
		start := s.r.pos
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if unknown && !isClusterChild(el.id) {
			return s.r.seek(start)
		}
		switch el.id {
		case idTimestamp:
			payload, err := s.r.readPayload(el)
			if err != nil {
				return err
			}
			clusterTS = parseUintBytes(payload)
		case idSimpleBlock, idBlockGroup:
			payload, err := s.r.readPayload(el)
			if errors.Is(err, errElementTooLarge) {
				if skipErr := s.r.skipElement(el); skipErr != nil {
					return skipErr
				}
				continue
			}
			if err != nil {
				return err
			}
			if el.id == idSimpleBlock {
				appendSimpleCue(payload, clusterTS, s.timestampScale, tracks, cues)
			} else {
				appendBlockGroupCue(payload, clusterTS, s.timestampScale, tracks, cues)
			}
		default:
			if err := s.r.skipElement(el); err != nil {
				return err
			}
		}
	}
	if s.r.pos < end && end >= 0 {
		return s.r.seek(end)
	}
	return nil
}

func appendSimpleCue(payload []byte, clusterTS, scale uint64, tracks map[uint64]mkvSubTrack, cues map[uint64][]mkvCue) {
	number, _, err := decodeBlockVINT(payload)
	if err != nil {
		return
	}
	track, ok := tracks[number]
	if !ok {
		return
	}
	if cue, ok := parseSimpleBlock(payload, clusterTS, scale, track); ok {
		cues[number] = append(cues[number], cue)
	}
}

func appendBlockGroupCue(payload []byte, clusterTS, scale uint64, tracks map[uint64]mkvSubTrack, cues map[uint64][]mkvCue) {
	var block []byte
	off := 0
	for off < len(payload) {
		id, idWidth, err := decodeElementID(payload[off:])
		if err != nil {
			return
		}
		off += idWidth
		size, sizeWidth, unknown, err := decodeElementSize(payload[off:])
		if err != nil || unknown || int(size) > len(payload)-off-sizeWidth {
			return
		}
		off += sizeWidth
		child := payload[off : off+int(size)]
		off += int(size)
		if id == idBlock {
			block = child
		}
	}
	if len(block) == 0 {
		return
	}
	number, _, err := decodeBlockVINT(block)
	if err != nil {
		return
	}
	track, ok := tracks[number]
	if !ok {
		return
	}
	if cue, ok := parseBlockGroup(payload, clusterTS, scale, track); ok {
		cues[number] = append(cues[number], cue)
	}
}

func (s *mkvSession) parseCluster(el mkvElement, track mkvSubTrack, cues *[]mkvCue) error {
	size := el.dataEnd - s.r.pos
	if !el.unknown && size >= 0 && size <= mkvMaxClusterBuffer {
		payload, err := s.r.readFull(int(size))
		if err != nil {
			return err
		}
		return parseClusterBytes(payload, s.timestampScale, track, cues)
	}
	return s.parseClusterStreaming(el.dataEnd, el.unknown, track, cues)
}

func isClusterChild(id uint64) bool {
	switch id {
	case idTimestamp, idSimpleBlock, idBlockGroup,
		0xAF,   // EncryptedBlock
		0xAB,   // PrevSize
		0xA7,   // Position
		0x5854, // SilentTracks
		0xBF,   // CRC-32
		0xEC:   // Void
		return true
	default:
		return false
	}
}

func (s *mkvSession) parseClusterStreaming(end int64, unknown bool, track mkvSubTrack, cues *[]mkvCue) error {
	var clusterTS uint64
	for {
		start := s.r.pos
		el, err := s.r.nextElement(end)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if unknown && !isClusterChild(el.id) {
			return s.r.seek(start)
		}
		switch el.id {
		case idTimestamp:
			payload, err := s.r.readPayload(el)
			if err != nil {
				return err
			}
			clusterTS = parseUintBytes(payload)
		case idSimpleBlock, idBlockGroup:
			payload, err := s.r.readPayload(el)
			if errors.Is(err, errElementTooLarge) {
				if skipErr := s.r.skipElement(el); skipErr != nil {
					return skipErr
				}
				continue
			}
			if err != nil {
				return err
			}
			if el.id == idSimpleBlock {
				if cue, ok := parseSimpleBlock(payload, clusterTS, s.timestampScale, track); ok {
					*cues = append(*cues, cue)
				}
			} else if cue, ok := parseBlockGroup(payload, clusterTS, s.timestampScale, track); ok {
				*cues = append(*cues, cue)
			}
		default:
			if err := s.r.skipElement(el); err != nil {
				return err
			}
		}
	}
	if s.r.pos < end && end >= 0 {
		return s.r.seek(end)
	}
	return nil
}

func parseClusterBytes(data []byte, scale uint64, track mkvSubTrack, cues *[]mkvCue) error {
	var clusterTS uint64
	off := 0
	for off < len(data) {
		id, idWidth, err := decodeElementID(data[off:])
		if err != nil {
			return err
		}
		off += idWidth
		size, sizeWidth, unknown, err := decodeElementSize(data[off:])
		if err != nil {
			return err
		}
		off += sizeWidth
		var payload []byte
		if unknown {
			payload = data[off:]
			off = len(data)
		} else {
			if int(size) > len(data)-off {
				return errors.New("mkv: Cluster 子元素越界")
			}
			payload = data[off : off+int(size)]
			off += int(size)
		}
		switch id {
		case idTimestamp:
			clusterTS = parseUintBytes(payload)
		case idSimpleBlock:
			if cue, ok := parseSimpleBlock(payload, clusterTS, scale, track); ok {
				*cues = append(*cues, cue)
			}
		case idBlockGroup:
			if cue, ok := parseBlockGroup(payload, clusterTS, scale, track); ok {
				*cues = append(*cues, cue)
			}
		}
	}
	return nil
}

func decodeElementID(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, io.EOF
	}
	width, err := vintWidth(data[0])
	if err != nil {
		return 0, 0, err
	}
	if len(data) < width {
		return 0, 0, io.ErrUnexpectedEOF
	}
	id := uint64(0)
	for i := 0; i < width; i++ {
		id = (id << 8) | uint64(data[i])
	}
	return id, width, nil
}

func decodeElementSize(data []byte) (uint64, int, bool, error) {
	if len(data) == 0 {
		return 0, 0, false, io.EOF
	}
	width, err := vintWidth(data[0])
	if err != nil {
		return 0, 0, false, err
	}
	if len(data) < width {
		return 0, 0, false, io.ErrUnexpectedEOF
	}
	mask := vintDataMask(width) - 1
	value := uint64(data[0] & mask)
	allOnes := data[0]&mask == mask
	for i := 1; i < width; i++ {
		value = (value << 8) | uint64(data[i])
		if data[i] != 0xFF {
			allOnes = false
		}
	}
	return value, width, allOnes, nil
}

func decodeBlockVINT(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, io.EOF
	}
	width, err := vintWidth(data[0])
	if err != nil {
		return 0, 0, err
	}
	if len(data) < width {
		return 0, 0, io.ErrUnexpectedEOF
	}
	mask := vintDataMask(width) - 1
	value := uint64(data[0] & mask)
	for i := 1; i < width; i++ {
		value = (value << 8) | uint64(data[i])
	}
	return value, width, nil
}

func parseSimpleBlock(payload []byte, clusterTS, scale uint64, track mkvSubTrack) (mkvCue, bool) {
	cue, frame, ok := parseBlockHeader(payload, clusterTS, scale, track)
	if !ok {
		return mkvCue{}, false
	}
	text := decodeCueText(track.codec, frame)
	if text == "" {
		return mkvCue{}, false
	}
	cue.end = cue.start + cueDuration(track, 0)
	cue.text = text
	return cue, true
}

func parseBlockGroup(payload []byte, clusterTS, scale uint64, track mkvSubTrack) (mkvCue, bool) {
	var (
		block      []byte
		durationTS uint64
		hasDur     bool
	)
	off := 0
	for off < len(payload) {
		id, idWidth, err := decodeElementID(payload[off:])
		if err != nil {
			return mkvCue{}, false
		}
		off += idWidth
		size, sizeWidth, unknown, err := decodeElementSize(payload[off:])
		if err != nil {
			return mkvCue{}, false
		}
		off += sizeWidth
		if unknown || int(size) > len(payload)-off {
			return mkvCue{}, false
		}
		child := payload[off : off+int(size)]
		off += int(size)
		switch id {
		case idBlock:
			block = child
		case idBlockDuration:
			durationTS = parseUintBytes(child)
			hasDur = true
		}
	}
	if len(block) == 0 {
		return mkvCue{}, false
	}
	cue, frame, ok := parseBlockHeader(block, clusterTS, scale, track)
	if !ok {
		return mkvCue{}, false
	}
	text := decodeCueText(track.codec, frame)
	if text == "" {
		return mkvCue{}, false
	}
	if hasDur {
		cue.end = cue.start + ticksToDuration(durationTS, scale)
	} else {
		cue.end = cue.start + cueDuration(track, 0)
	}
	if cue.end <= cue.start {
		cue.end = cue.start + cueDuration(track, 0)
	}
	cue.text = text
	return cue, true
}

func parseBlockHeader(payload []byte, clusterTS, scale uint64, track mkvSubTrack) (mkvCue, []byte, bool) {
	number, width, err := decodeBlockVINT(payload)
	if err != nil || number != track.number || len(payload) < width+3 {
		return mkvCue{}, nil, false
	}
	rel := int16(binary.BigEndian.Uint16(payload[width : width+2]))
	flags := payload[width+2]
	if (flags>>1)&0x03 != 0 {
		// 字幕轨几乎不使用 lacing；遇到时跳过以免把长度头当文本。
		return mkvCue{}, nil, false
	}
	frame := payload[width+3:]
	startTicks := int64(clusterTS) + int64(rel)
	if startTicks < 0 {
		startTicks = 0
	}
	return mkvCue{start: ticksToDuration(uint64(startTicks), scale)}, frame, true
}

func cueDuration(track mkvSubTrack, explicit time.Duration) time.Duration {
	if explicit > 0 {
		return explicit
	}
	if track.defaultDuration > 0 {
		return track.defaultDuration
	}
	return mkvDefaultCueDuration
}

func ticksToDuration(ticks, scale uint64) time.Duration {
	if ticks == 0 {
		return 0
	}
	if scale == 0 {
		scale = mkvDefaultTimestampNs
	}
	if ticks > uint64(math.MaxInt64)/scale {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(ticks * scale)
}

func payloadToString(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if utf8.Valid(data) {
		return string(data)
	}
	text, err := decodeText(data, detectTextEncoding(data))
	if err != nil {
		return string(data)
	}
	return text
}

func decodeCueText(codec string, payload []byte) string {
	raw := strings.TrimRight(payloadToString(payload), "\x00")
	raw = strings.TrimPrefix(raw, "\uFEFF")
	switch codec {
	case "S_TEXT/ASS", "S_TEXT/SSA":
		return assDialogueText(raw)
	default:
		return normalizeSubtitleText(raw)
	}
}

func normalizeSubtitleText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\\N", "\n")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\h", " ")
	return strings.TrimSpace(s)
}

func assDialogueText(s string) string {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "Dialogue:"); ok {
		s = strings.TrimSpace(rest)
		parts := strings.SplitN(s, ",", 10)
		if len(parts) >= 10 {
			s = parts[9]
		}
	} else {
		parts := strings.SplitN(s, ",", 9)
		if len(parts) >= 9 {
			s = parts[8]
		}
	}
	return normalizeSubtitleText(stripASSOverrides(s))
}

func stripASSOverrides(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '{' {
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func cuesToWebVTT(cues []mkvCue) (string, error) {
	if len(cues) == 0 {
		return "", errors.New("该内嵌字幕轨道没有可显示的文本")
	}
	var b strings.Builder
	b.WriteString("WEBVTT\n")
	for _, cue := range cues {
		if cue.end <= cue.start {
			cue.end = cue.start + mkvDefaultCueDuration
		}
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s --> %s\n%s\n", formatVTTTime(cue.start), formatVTTTime(cue.end), cue.text)
		if b.Len() > mkvMaxExtractedVTTSize {
			return "", fmt.Errorf("内嵌字幕过大（超过 %dMB）", mkvMaxExtractedVTTSize>>20)
		}
	}
	return b.String(), nil
}

func formatVTTTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalMs := d.Milliseconds()
	hours := totalMs / 3_600_000
	minutes := (totalMs % 3_600_000) / 60_000
	seconds := (totalMs % 60_000) / 1_000
	ms := totalMs % 1_000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
}

var languageDisplayNames = map[string]string{
	"chi": "中文",
	"zho": "中文",
	"zh":  "中文",
	"chs": "中文",
	"cht": "中文",
	"eng": "English",
	"en":  "English",
	"jpn": "日本語",
	"ja":  "日本語",
	"kor": "한국어",
	"ko":  "한국어",
	"fra": "Français",
	"fre": "Français",
	"fr":  "Français",
	"deu": "Deutsch",
	"ger": "Deutsch",
	"de":  "Deutsch",
	"spa": "Español",
	"es":  "Español",
	"rus": "Русский",
	"ru":  "Русский",
	"ita": "Italiano",
	"it":  "Italiano",
	"por": "Português",
	"pt":  "Português",
}

func displayLanguage(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || strings.EqualFold(code, "und") {
		return ""
	}
	lower := strings.ToLower(code)
	if name, ok := languageDisplayNames[lower]; ok {
		return name
	}
	if i := strings.IndexByte(lower, '-'); i > 0 {
		if name, ok := languageDisplayNames[lower[:i]]; ok {
			return name
		}
	}
	return code
}

func subtitleTrackLabel(track mkvSubTrack, index int) string {
	name := strings.TrimSpace(track.name)
	lang := displayLanguage(track.language)
	switch {
	case name != "" && lang != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(lang)):
		return name + " (" + lang + ")"
	case name != "":
		return name
	case lang != "":
		return lang
	default:
		return fmt.Sprintf("内嵌字幕 %d", index)
	}
}
