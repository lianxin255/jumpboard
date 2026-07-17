// glassTheme.ts — iOS 27 Liquid Glass 官方玻璃主题（antdv-style createStyles + ConfigProvider 注入）
// 基于 antdv-next 官网 glassTheme 方案，适配 antdv-style@0.1.0-alpha.1 的 API（cssVar → token）
import type { ButtonProps, ConfigProviderProps } from 'antdv-next'
import { clsx } from '@v-c/util'
import { theme } from 'antdv-next'
import { createStyles } from 'antdv-style'
import { computed } from 'vue'

const useStyles = createStyles(({ token, css }) => {
  const glassBorder = {
    boxShadow: [
      '0 8px 24px rgba(85, 85,85, 0.1)',
      'inset 0 0 5px 2px rgba(255, 255, 255, 0.3)',
      'inset 0 5px 2px rgba(255, 255, 255, 0.2)',
    ].join(','),
  }

  const glassBox = {
    ...glassBorder,
    background: `color-mix(in srgb, ${token.colorBgContainer} 15%, transparent)`,
    backdropFilter: 'blur(12px)',
  }

  return {
    glassBorder,
    glassBox,
    notBackdropFilter: css({
      backdropFilter: 'none',
    }),
    app: css({
      textShadow: '0 1px rgba(0,0,0,0.1)',
    }),
    cardRoot: css({
      ...glassBox,
      backgroundColor: `color-mix(in srgb, ${token.colorBgContainer} 40%, transparent)`,
    }),
    modalContainer: css({
      ...glassBox,
      backdropFilter: 'none',
    }),
    buttonRoot: css({
      ...glassBorder,
    }),
    buttonRootDefaultColor: css({
      background: 'transparent',
      color: token.colorText,

      '&:hover': {
        background: 'rgba(255,255,255,0.2)',
        color: `color-mix(in srgb, ${token.colorText} 90%, transparent)`,
      },

      '&:active': {
        background: 'rgba(255,255,255,0.1)',
        color: `color-mix(in srgb, ${token.colorText} 80%, transparent)`,
      },
    }),
    buttonRootDangerColor: css({
      background: 'rgba(255, 120, 117, 0.1)',
      borderColor: 'rgba(255, 120, 117, 0.24)',
      color: token.colorError,

      '&:hover': {
        background: 'rgba(255, 120, 117, 0.16)',
        borderColor: 'rgba(255, 120, 117, 0.32)',
        color: token.colorErrorHover,
      },

      '&:active': {
        background: 'rgba(255, 120, 117, 0.12)',
        borderColor: 'rgba(255, 120, 117, 0.28)',
        color: token.colorErrorActive,
      },
    }),

    dropdownRoot: css({
      ...glassBox,
      borderRadius: token.borderRadiusLG,

      ul: {
        background: 'transparent',
      },
    }),
    notificationRoot: css({
      '&.ant-notification-notice, & .ant-notification-notice': {
        ...glassBox,
        background: `color-mix(in srgb, ${token.colorBgContainer} 25%, transparent)`,
      },
    }),
    switchRoot: css({ ...glassBorder, border: 'none' }),
    segmentedRoot: css({
      ...glassBorder,
      background: 'transparent',
      backdropFilter: 'none',

      '& .ant-segmented-thumb': {
        ...glassBox,
      },

      '& .ant-segmented-item-selected': {
        ...glassBox,
      },
    }),
    radioButtonRoot: css({
      '&.ant-radio-button-wrapper': {
        ...glassBorder,
        background: 'transparent',
        borderColor: 'rgba(255, 255, 255, 0.2)',
        color: token.colorText,

        '&:hover': {
          borderColor: 'rgba(255, 255, 255, 0.24)',
          color: token.colorText,
        },

        '&.ant-radio-button-wrapper-checked:not(.ant-radio-button-wrapper-disabled)': {
          ...glassBox,
          borderColor: 'rgba(255, 255, 255, 0.28)',
          color: token.colorText,

          '&::before': {
            backgroundColor: 'rgba(255, 255, 255, 0.18)',
          },

          '&:hover': {
            color: token.colorText,
          },
        },
      },
    }),
    progressTrack: css({
      ...glassBorder,
      height: '12px',
    }),
    progressRail: css({
      height: '12px',
    }),
  }
})

function useGlassTheme() {
  const { styles } = useStyles()

  return computed<ConfigProviderProps>(() => ({
    theme: {
      algorithm: theme.defaultAlgorithm,
      token: {
        // iOS 27 系统蓝（微调：原官网用 antd 默认蓝，改成 iOS 蓝）
        colorPrimary: '#0A84FF',
        borderRadius: 12,
        borderRadiusLG: 12,
        borderRadiusSM: 12,
        borderRadiusXS: 12,
        // iOS 27 SF 字体栈（微调）
        fontFamily: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
        motionDurationSlow: '0.2s',
        motionDurationMid: '0.1s',
        motionDurationFast: '0.05s',
      },
      components: {
        Button: {
          primaryShadow: 'none',
          dangerShadow: 'none',
          defaultShadow: 'none',
          colorError: '#ff7875',
          colorErrorHover: '#ffa39e',
          colorErrorActive: '#ff4d4f',
          colorErrorBg: 'rgba(255, 120, 117, 0.1)',
          colorErrorBgFilledHover: 'rgba(255, 120, 117, 0.16)',
          colorErrorBgActive: 'rgba(255, 120, 117, 0.12)',
          colorErrorBorder: 'rgba(255, 120, 117, 0.24)',
          colorErrorBorderHover: 'rgba(255, 120, 117, 0.32)',
          colorErrorText: '#ff7875',
          colorErrorTextHover: '#ffa39e',
          colorErrorTextActive: '#ff4d4f',
          defaultBg: 'rgba(255, 255, 255, 0.1)',
          defaultBorderColor: 'rgba(255, 255, 255, 0.1)',
          defaultHoverBg: 'rgba(255, 255, 255, 0.2)',
          defaultHoverBorderColor: 'rgba(255, 255, 255, 0.1)',
          defaultActiveBg: 'rgba(255, 255, 255, 0.1)',
          defaultActiveBorderColor: 'rgba(255, 255, 255, 0.18)',
        },
        Notification: {
          colorSuccessBg: 'rgba(183, 235, 143, 0.18)',
          colorErrorBg: 'rgba(255, 120, 117, 0.16)',
          colorInfoBg: 'rgba(145, 202, 255, 0.18)',
          colorWarningBg: 'rgba(255, 229, 143, 0.18)',
        },
        Layout: {
          bodyBg: 'rgba(255, 255, 255, 0.12)',
          footerBg: 'rgba(255, 255, 255, 0.12)',
          headerBg: 'rgba(255, 255, 255, 0.32)',
          headerColor: 'rgba(0, 0, 0, 0.88)',
          siderBg: 'rgba(255, 255, 255, 0.18)',
          triggerBg: 'rgba(255, 255, 255, 0.28)',
          triggerColor: 'rgba(0, 0, 0, 0.88)',
        },
        Menu: {
          activeBarBorderWidth: 0,
          groupTitleColor: 'rgba(0, 0, 0, 0.55)',
          itemActiveBg: 'rgba(255, 255, 255, 0.24)',
          itemBg: 'transparent',
          itemColor: 'rgba(0, 0, 0, 0.78)',
          itemHoverBg: 'rgba(255, 255, 255, 0.28)',
          itemHoverColor: 'rgba(0, 0, 0, 0.88)',
          itemSelectedBg: 'color-mix(in srgb, #0A84FF 18%, transparent)',
          itemSelectedColor: '#0A84FF',
          subMenuItemBg: 'transparent',
          subMenuItemSelectedColor: 'rgba(0, 0, 0, 0.88)',
        },
        Alert: {},
        Modal: {},
        Card: {},
        Tooltip: {},
        Checkbox: {},
        Radio: {},
        Select: {},
        Input: {},
        Switch: {},
        Progress: {
          circleTextColor: 'rgba(0, 0, 0, 0.88)',
          defaultColor: '#0A84FF',
          remainingColor: 'rgba(255, 255, 255, 0.28)',
        },
        Steps: {},
        Slider: {},
        ColorPicker: {},
      },
    },
    app: {
      class: styles.app,
    },
    card: {
      classes: {
        root: styles.cardRoot,
      },
    },
    modal: {
      classes: {
        container: styles.modalContainer,
      },
    },
    button: {
      classes: ({ props }: { props: ButtonProps }) => ({
        root: clsx(
          styles.buttonRoot,
          props.color === 'default' && styles.buttonRootDefaultColor,
          props.color === 'danger' && styles.buttonRootDangerColor,
        ),
      }),
    },
    alert: {
      class: clsx(styles.glassBox, styles.notBackdropFilter),
    },
    colorPicker: {
      arrow: false,
      classes: {
        root: clsx(styles.glassBox, styles.notBackdropFilter),
      },
    },
    dropdown: {
      classes: {
        root: styles.dropdownRoot,
      },
    },
    select: {
      classes: {
        root: clsx(styles.glassBox, styles.notBackdropFilter),
        popup: {
          root: styles.glassBox,
        },
      },
    },
    datePicker: {
      classes: {
        root: clsx(styles.glassBox, styles.notBackdropFilter),
        popup: {
          container: styles.glassBox,
        },
      },
    },
    input: {
      classes: {
        root: clsx(styles.glassBox, styles.notBackdropFilter),
      },
    },
    inputNumber: {
      classes: {
        root: clsx(styles.glassBox, styles.notBackdropFilter),
      },
    },
    popover: {
      classes: {
        container: styles.glassBox,
      },
    },
    notification: {
      classes: {
        root: styles.notificationRoot,
      },
    },
    switch: {
      classes: {
        root: styles.switchRoot,
      },
    },
    radio: {
      classes: {
        root: styles.radioButtonRoot,
      },
    },
    segmented: {
      class: styles.segmentedRoot,
    },
    progress: {
      classes: {
        track: styles.glassBorder,
      },
      styles: {
        track: {
          height: '12px',
        },
        rail: {
          height: '12px',
        },
      },
    },
    wave: {},
    checkbox: {},
    tooltip: {},
  }))
}

export default useGlassTheme
