import { createApp } from "vue";
import {
  LayButton,
  LayEmpty,
  LayIcon,
  LayLoading,
  LayTab,
  LayTabItem,
} from "@layui/layui-vue";
import "@layui/layui-vue/es/index/index.css";
import "@layui/layui-vue/es/button/index.css";
import "@layui/layui-vue/es/empty/index.css";
import "@layui/layui-vue/es/loading/index.css";
import "@layui/layui-vue/es/tab/index.css";
import App from "./App.vue";
import "./theme.css";
import "./style.css";

createApp(App)
  .use(LayButton)
  .use(LayEmpty)
  .use(LayIcon)
  .use(LayLoading)
  .use(LayTab)
  .use(LayTabItem)
  .mount("#app");
