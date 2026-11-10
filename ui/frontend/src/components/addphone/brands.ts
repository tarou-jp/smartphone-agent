// Android phones by maker, after ui/brands.go: where each keeps
// Developer options and the switches it needs. Values are locale keys.

export interface BrandSwitch {
  name: string;
  why?: string;
  path?: string;
  off?: boolean;
}

export interface Brand {
  id: string;
  name: string;
  about: string;
  tap: string;
  aboutNote?: string;
  dev: string;
  note?: string;
  switches: BrandSwitch[];
  promptAlways?: string;
  promptAllow?: string;
}

const usb: BrandSwitch = { name: 'guideSwitchUSBDebugging' };

export const brands: Brand[] = [
  {
    id: 'xiaomi',
    name: 'brandXiaomi',
    about: 'guideXiaomiAbout',
    tap: 'guideXiaomiTap',
    aboutNote: 'guideXiaomiAboutNote',
    dev: 'guideXiaomiDev',
    note: 'guideXiaomiNote',
    switches: [
      { name: 'guideXiaomiSwitchUSB' },
      { name: 'guideXiaomiSwitchSecurity', why: 'guideXiaomiSwitchSecurityWhy' },
      { name: 'guideXiaomiSwitchInstall', why: 'guideXiaomiSwitchInstallWhy' },
    ],
  },
  {
    id: 'oppo',
    name: 'brandOppo',
    about: 'guideOppoAbout',
    tap: 'guideOppoTap',
    aboutNote: 'guideOppoAboutNote',
    dev: 'guideOppoDev',
    note: 'guideOppoNote',
    switches: [usb, { name: 'guideOppoSwitchMonitor', why: 'guideOppoSwitchMonitorWhy' }],
  },
  {
    id: 'vivo',
    name: 'brandVivo',
    about: 'guideVivoAbout',
    tap: 'guideVivoTap',
    aboutNote: 'guideVivoAboutNote',
    dev: 'guideVivoDev',
    note: 'guideVivoNote',
    switches: [usb],
  },
  {
    id: 'honor',
    name: 'brandHonor',
    about: 'guideHonorAbout',
    tap: 'guideHonorTap',
    aboutNote: 'guideHonorAboutNote',
    dev: 'guideHonorDev',
    note: 'guideHonorNote',
    promptAlways: 'guideHonorPromptAlways',
    promptAllow: 'guidePromptOK',
    switches: [{ name: 'guideHonorSwitchCharge', why: 'guideHonorSwitchChargeWhy' }, usb],
  },
  {
    id: 'huawei',
    name: 'brandHuawei',
    about: 'guideHuaweiAbout',
    tap: 'guideHuaweiTap',
    aboutNote: 'guideHuaweiAboutNote',
    dev: 'guideHuaweiDev',
    note: 'guideHuaweiNote',
    promptAlways: 'guideHuaweiPromptAlways',
    promptAllow: 'guidePromptOK',
    switches: [
      { name: 'guideHuaweiSwitchCharge', why: 'guideHuaweiSwitchChargeWhy' },
      usb,
      { name: 'guideHuaweiSwitchMonitor', why: 'guideHuaweiSwitchMonitorWhy', off: true },
    ],
  },
  {
    id: 'samsung',
    name: 'brandSamsung',
    about: 'guideSamsungAbout',
    tap: 'guideSamsungTap',
    dev: 'guideSamsungDev',
    note: 'guideSamsungNote',
    switches: [
      { name: 'guideSamsungAutoBlocker', why: 'guideSamsungAutoBlockerWhy', path: 'guideSamsungAutoBlockerPath', off: true },
      usb,
    ],
  },
  {
    id: 'pixel',
    name: 'brandPixel',
    about: 'guidePixelAbout',
    tap: 'guidePixelTap',
    aboutNote: 'guidePixelAboutNote',
    dev: 'guidePixelDev',
    switches: [usb],
  },
  {
    id: 'other',
    name: 'brandOther',
    about: 'guideOtherAbout',
    tap: 'guideOtherTap',
    aboutNote: 'guideOtherAboutNote',
    dev: 'guideOtherDev',
    note: 'guideOtherNote',
    switches: [usb],
  },
];

export function brandByID(id: string): Brand {
  return brands.find((b) => b.id === id) ?? brands[brands.length - 1];
}
