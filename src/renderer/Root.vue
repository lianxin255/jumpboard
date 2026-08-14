<script setup lang="ts">
// 根组件。antdv-next 的 ThemeProvider/ConfigProvider 主题配置已移除——
// 项目 UI 全部自绘（tokens.css + 各组件 scoped style），从未使用 <a-*> 组件，
// ConfigProvider 的 175 行主题 token 无消费方，是死代码。
import App from './App.vue'
</script>

<template>
  <App />
</template>
