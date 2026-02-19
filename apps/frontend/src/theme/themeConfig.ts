import type { ThemeConfig } from 'antd';
import { theme } from 'antd';

export const cohesionPalette = {
  ink_black: {
    DEFAULT: '#0d1b2a',
    100: '#030609',
    200: '#050b11',
    300: '#08111a',
    400: '#0b1622',
    500: '#0d1b2a',
    600: '#234870',
    700: '#3875b6',
    800: '#74a3d4',
    900: '#bad1ea',
  },
  prussian_blue: {
    DEFAULT: '#1b263b',
    100: '#05080c',
    200: '#0b0f18',
    300: '#101724',
    400: '#161f30',
    500: '#1b263b',
    600: '#364c75',
    700: '#5172af',
    800: '#8ba1ca',
    900: '#c5d0e4',
  },
  dusk_blue: {
    DEFAULT: '#415a77',
    100: '#0d1218',
    200: '#1a242f',
    300: '#273647',
    400: '#34485f',
    500: '#415a77',
    600: '#587aa1',
    700: '#819bb9',
    800: '#abbcd1',
    900: '#d5dee8',
  },
  dusty_denim: {
    DEFAULT: '#778da9',
    100: '#161c23',
    200: '#2c3746',
    300: '#425369',
    400: '#586f8d',
    500: '#778da9',
    600: '#91a2ba',
    700: '#acbacb',
    800: '#c8d1dc',
    900: '#e3e8ee',
  },
  alabaster_grey: {
    DEFAULT: '#e0e1dd',
    100: '#2e2f2a',
    200: '#5b5e53',
    300: '#898c7e',
    400: '#b4b6ad',
    500: '#e0e1dd',
    600: '#e5e6e3',
    700: '#ececea',
    800: '#f2f3f1',
    900: '#f9f9f8',
  },
} as const;

const sharedToken: ThemeConfig['token'] = {
  colorPrimary: cohesionPalette.dusk_blue.DEFAULT,
  colorInfo: cohesionPalette.dusk_blue.DEFAULT,
  colorLink: cohesionPalette.prussian_blue[700],
  borderRadius: 8,
};

type SelectionTokenSet = {
  bg: string;
  hoverBg: string;
  text: string;
};

const lightSelectionToken: SelectionTokenSet = {
  bg: cohesionPalette.dusk_blue[900],
  hoverBg: cohesionPalette.alabaster_grey[700],
  text: cohesionPalette.prussian_blue.DEFAULT,
};

const darkSelectionToken: SelectionTokenSet = {
  bg: cohesionPalette.dusk_blue[300],
  hoverBg: cohesionPalette.prussian_blue[500],
  text: cohesionPalette.alabaster_grey[900],
};

const lightToken: ThemeConfig['token'] = {
  colorBgBase: cohesionPalette.alabaster_grey[900],
  colorBgLayout: cohesionPalette.alabaster_grey[800],
  colorBgContainer: cohesionPalette.alabaster_grey[900],
  colorBgElevated: cohesionPalette.alabaster_grey[900],
  colorTextBase: cohesionPalette.prussian_blue.DEFAULT,
  colorBorder: cohesionPalette.dusty_denim[800],
  colorBorderSecondary: cohesionPalette.alabaster_grey[600],
  controlItemBgActive: lightSelectionToken.bg,
  controlItemBgHover: lightSelectionToken.hoverBg,
};

const darkToken: ThemeConfig['token'] = {
  colorBgBase: cohesionPalette.ink_black.DEFAULT,
  colorBgLayout: cohesionPalette.ink_black[300],
  colorBgContainer: cohesionPalette.prussian_blue[400],
  colorBgElevated: cohesionPalette.prussian_blue[500],
  colorTextBase: cohesionPalette.alabaster_grey.DEFAULT,
  colorBorder: cohesionPalette.dusk_blue[400],
  colorBorderSecondary: cohesionPalette.prussian_blue[600],
  controlItemBgActive: darkSelectionToken.bg,
  controlItemBgHover: darkSelectionToken.hoverBg,
};

export function buildCohesionThemeConfig(isDarkMode: boolean): ThemeConfig {
  const selectionToken = isDarkMode ? darkSelectionToken : lightSelectionToken;

  return {
    cssVar: {},
    algorithm: isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      ...sharedToken,
      ...(isDarkMode ? darkToken : lightToken),
    },
    components: {
      Menu: {
        itemSelectedBg: selectionToken.bg,
        itemSelectedColor: selectionToken.text,
        subMenuItemSelectedColor: selectionToken.text,
        itemHoverBg: selectionToken.hoverBg,
        itemActiveBg: selectionToken.bg,
      },
      Tree: {
        nodeSelectedBg: selectionToken.bg,
        nodeSelectedColor: selectionToken.text,
        directoryNodeSelectedBg: selectionToken.bg,
        directoryNodeSelectedColor: selectionToken.text,
        nodeHoverBg: selectionToken.hoverBg,
      },
    },
  };
}
