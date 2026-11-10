// Words for the gateway's rule, action and category names, after
// ui/i18n.go and ui/settings.go.

import type { T } from '@/i18n';

const ruleActionKeys: Record<string, string> = {
  Payments: 'ruleActionPayments',
  Messages: 'ruleActionMessages',
  Commits: 'ruleActionCommits',
  'Installs and links': 'ruleActionInstallsLinks',
  Calls: 'ruleActionCalls',
  'Settings changes': 'ruleActionSettingsChanges',
  'Every action': 'ruleActionEveryAction',
  'Agent request': 'ruleActionAgentRequest',
};

/** What a rule covers, inside "Always allow payments in Dinely". */
export function ruleAction(t: T, rule: string): string {
  if (!rule) return t('ruleActionThisKind');
  const k = ruleActionKeys[rule];
  if (k) return t(k);
  return t('ruleActionOther', { rule: rule.charAt(0).toLowerCase() + rule.slice(1) });
}

const ruleNameKeys: Record<string, string> = {
  Payments: 'rulePayments',
  Messages: 'ruleMessages',
  Commits: 'ruleCommits',
  'Installs and links': 'ruleInstallsLinks',
  Calls: 'ruleCalls',
  'Settings changes': 'ruleSettingsChanges',
  'Every action': 'ruleEveryAction',
  'Agent request': 'ruleAgentRequest',
  'Finance apps': 'ruleFinanceApps',
  'Password managers': 'rulePasswordManagers',
  'Blocked by you': 'ruleBlockedByYou',
  'Read-only': 'ruleReadOnly',
  Files: 'ruleFiles',
  'Task apps': 'ruleTaskApps',
  'Agent scope': 'ruleAgentScope',
};

/** A rule's name in the person's language, for Pinky's own rules. */
export function ruleName(t: T, name: string): string {
  const k = ruleNameKeys[name];
  return k ? t(k) : name;
}

export const actionKeys: Record<string, string> = {
  commit: 'settingsRulesActionCommit',
  payment: 'settingsRulesActionPayment',
  send_message: 'settingsRulesActionSendMessage',
  install_app: 'settingsRulesActionInstallApp',
  open_url: 'settingsRulesActionOpenURL',
  call: 'settingsRulesActionCall',
  change_setting: 'settingsRulesActionChangeSetting',
  tap: 'settingsRulesActionTap',
  type: 'settingsRulesActionType',
  swipe: 'settingsRulesActionSwipe',
  key: 'settingsRulesActionKey',
  launch: 'settingsRulesActionLaunch',
};
export const actions = Object.keys(actionKeys);

export const categoryKeys: Record<string, string> = {
  finance: 'settingsSafetyKindFinance',
  passwords: 'settingsSafetyKindPasswords',
  messaging: 'settingsSafetyKindMessaging',
  social: 'settingsSafetyKindSocial',
  stores: 'settingsSafetyKindStores',
  browsers: 'settingsSafetyKindBrowsers',
  phone: 'settingsSafetyKindPhone',
  settings: 'settingsSafetyKindSettings',
  system: 'settingsSafetyKindSystem',
  blocked: 'settingsRulesBlockedDesc',
  credentials: 'settingsRulesCredentials',
};
export const categories = ['finance', 'passwords', 'messaging', 'social', 'stores', 'browsers', 'phone', 'settings', 'system', 'blocked'];

const kindKeys: Record<string, string> = {
  Finance: 'settingsSafetyKindFinance',
  Passwords: 'settingsSafetyKindPasswords',
  Banking: 'settingsSafetyKindBanking',
  Wallet: 'settingsSafetyKindWallet',
  Messaging: 'settingsSafetyKindMessaging',
  Social: 'settingsSafetyKindSocial',
  Stores: 'settingsSafetyKindStores',
  Browsers: 'settingsSafetyKindBrowsers',
  Phone: 'settingsSafetyKindPhone',
  Settings: 'settingsSafetyKindSettings',
  System: 'settingsSafetyKindSystem',
};

/** A blocked app's kind ("Banking") in the person's language. */
export function kindLabel(t: T, kind?: string): string {
  if (!kind) return '';
  const k = kindKeys[kind];
  return k ? t(k) : kind;
}

/** "Dinely, Chatter and Calendar". */
export function joinAnd(t: T, s: string[]): string {
  if (s.length === 0) return '';
  if (s.length === 1) return s[0];
  if (s.length === 2) return t('listPair', { first: s[0], second: s[1] });
  return t('listMany', { rest: s.slice(0, -1).join(t('listSeparator')), last: s[s.length - 1] });
}

export const splitList = (s: string) =>
  s
    .split(/[,、，]/)
    .map((x) => x.trim())
    .filter(Boolean);
