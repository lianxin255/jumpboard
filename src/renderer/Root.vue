<script setup lang="ts">
import { ThemeProvider, createStyles } from 'antdv-style'
import { ConfigProvider, theme } from 'antdv-next'
import type { ConfigProviderProps } from 'antdv-next'
import { clsx } from '@v-c/util'
import { computed } from 'vue'

// 暗色 token 上下文必须在 ThemeProvider 内部使用，所以拆成内部组件。
// 已去除毛玻璃注入，改由 antd darkAlgorithm + 自定义暗色 token 驱动。

const useStyles = createStyles(({ css }) => {
  return {
    // 应用根：轻微文字阴影提升暗底可读性
    app: css({ textShadow: '0 1px rgba(0,0,0,0.3)' }),
    // antd 组件的统一暗色细边框（替代旧的玻璃高光）
    borderSkin: css({
      borderColor: 'var(--border-soft) !important',
      boxShadow: 'none',
    }),
    // 弹窗/卡片容器统一暗色 surface 背景
    surfaceBg: css({
      background: 'var(--surface)',
      borderColor: 'var(--border-soft)',
      boxShadow: 'var(--shadow-md)',
    }),
  }
})

import { defineComponent, h } from 'vue'
import App from './App.vue'

const DarkConfigProvider = defineComponent({
  setup(_, { slots }) {
    const { styles } = useStyles()
    const configProps = computed<ConfigProviderProps>(() => ({
      theme: {
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#c96442',
          colorSuccess: '#5b9a4a',
          colorWarning: '#c9a23a',
          colorError: '#b53333',
          colorInfo: '#c96442',
          // 背景
          colorBgBase: '#141413',
          colorBgContainer: '#262624',
          colorBgElevated: '#30302e',
          colorBgLayout: '#141413',
          colorBgSpotlight: '#30302e',
          colorBgBlur: '#141413',
          // 文字
          colorTextBase: '#faf9f5',
          colorText: '#faf9f5',
          colorTextSecondary: '#b0aea5',
          colorTextTertiary: '#87867f',
          colorTextQuaternary: '#828179',
          // 边框
          colorBorder: '#3d3d3a',
          colorBorderSecondary: '#30302e',
          // 圆角缩小到设计稿风格
          borderRadius: 3,
          borderRadiusLG: 6,
          borderRadiusSM: 2,
          borderRadiusXS: 2,
          // 字体
          fontFamily: "Arial, system-ui, -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif",
          fontFamilyCode: "ui-monospace, 'JetBrains Mono', Menlo, Consolas, monospace",
          fontSize: 13,
          // 动效
          motionDurationSlow: '0.2s cubic-bezier(0.2,0,0,1)',
          motionDurationMid: '0.14s cubic-bezier(0.2,0,0,1)',
          motionDurationFast: '0.1s cubic-bezier(0.2,0,0,1)',
          // 阴影
          boxShadowSecondary: '0 1px 0 rgba(0,0,0,0.4), 0 8px 24px rgba(0,0,0,0.45)',
        },
        components: {
          Button: {
            primaryShadow: 'none',
            defaultShadow: 'none',
            dangerShadow: 'none',
            fontWeight: 500,
            controlHeight: 30,
          },
          Modal: {
            contentBg: '#30302e',
            headerBg: '#30302e',
            titleColor: '#faf9f5',
          },
          Input: {
            colorBgContainer: '#262624',
            activeBorderColor: '#c96442',
            hoverBorderColor: '#87867f',
          },
          InputNumber: {
            colorBgContainer: '#262624',
            activeBorderColor: '#c96442',
          },
          Select: {
            colorBgContainer: '#262624',
            optionSelectedBg: 'rgba(201,100,66,0.12)',
            optionActiveBg: '#3d3d3a',
          },
          Tabs: {
            cardBg: 'transparent',
            cardActiveColor: '#faf9f5',
            itemColor: '#87867f',
            itemHoverColor: '#b0aea5',
            itemSelectedColor: '#faf9f5',
            inkBarColor: '#c96442',
          },
          Switch: {
            colorPrimary: '#5b9a4a',
          },
          Card: {
            colorBgContainer: '#262624',
            headerBg: 'transparent',
          },
          Segmented: {
            itemSelectedBg: '#3d3d3a',
            trackBg: '#262624',
            itemColor: '#87867f',
          },
          Tag: {
            defaultBg: '#262624',
          },
          Popover: {
            colorBgElevated: '#30302e',
          },
          Tooltip: {
            colorBgSpotlight: '#30302e',
          },
          Table: {
            headerBg: '#1c1c1a',
            rowHoverBg: '#262624',
          },
        },
      },
      app: { class: styles.app },
      // 弹窗/下拉统一暗色 surface 背景 + 细边框
      modal: { classes: { container: styles.surfaceBg } },
      card: { classes: { root: styles.borderSkin } },
      dropdown: { classes: { root: styles.surfaceBg } },
      select: {
        classes: {
          root: styles.borderSkin,
          popup: { root: styles.surfaceBg },
        },
      },
      input: { classes: { root: styles.borderSkin } },
      inputNumber: { classes: { root: styles.borderSkin } },
      popover: { classes: { container: styles.surfaceBg } },
      notification: { classes: { root: styles.surfaceBg } },
      colorPicker: {
        arrow: false,
        classes: { root: styles.borderSkin },
      },
      datePicker: {
        classes: {
          root: styles.borderSkin,
          popup: { container: styles.surfaceBg },
        },
      },
    }))
    return () => h(ConfigProvider, configProps.value, slots)
  },
})
</script>

<template>
  <ThemeProvider>
    <DarkConfigProvider>
      <App />
    </DarkConfigProvider>
  </ThemeProvider>
</template>
