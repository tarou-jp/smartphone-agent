// The gateway words steps, approvals and status lines in English as it
// records them, and history keeps those words. In another language the
// client reads them back by their patterns and says them in its own words;
// text that matches none, and in English all of it, shows as the gateway
// wrote it. A port of ui/gwtext.go; keep the two in step.

import { useMemo } from 'react';
import { useI18n, type T } from '@/i18n';
import type { Device, Need, ScopeView } from '@/api/types';

const reTapLabel = /^(Tapped|Long-pressed) “(.*)”$/;
const reTapAt = /^(Tapped|Long-pressed) at (-?\d+),(-?\d+)$/;
const reTapBare = /^(Tapped|Long-pressed)$/;
const reTyped = /^Typed “(.*?)”(?: into “(.*)”)?( and submitted)?$/;
const reSwipeIn = /^Swiped (\S+) in “(.*)”$/;
const reSwipeFrom = /^Swiped from (-?\d+),(-?\d+) to (-?\d+),(-?\d+)$/;
const reSwipe = /^Swiped (\S+)$/;
const rePressed = /^Pressed (.+)$/;
const reOpened = /^Opened (.+)$/;
const reCopyOne = /^[Cc]opy “(.*)” to (.+)$/;
const reCopyMany = /^[Cc]opy (\d+) files? to (.+)$/;
const reListFolder = /^[Ll]ist the phone's Pinky folder$/;
const reCopyFrom = /^[Cc]opy from the phone$/;
const reCopyTo = /^[Cc]opy to the phone$/;

interface Action {
  verb: string;
  label?: string;
  text?: string;
  x?: string;
  y?: string;
  x2?: string;
  y2?: string;
  dir?: string;
  key?: string;
  app?: string;
  files?: string;
  phone?: string;
  into?: boolean;
  submit?: boolean;
  placed?: boolean;
}

const tapVerb = (s: string) => (s === 'Long-pressed' ? 'longpress' : 'tap');

function parseAction(s: string): Action | null {
  let m;
  if ((m = reTapLabel.exec(s))) return { verb: tapVerb(m[1]), label: m[2], into: true };
  if ((m = reTapAt.exec(s))) return { verb: tapVerb(m[1]), x: m[2], y: m[3], placed: true };
  if ((m = reTapBare.exec(s))) return { verb: tapVerb(m[1]) };
  if ((m = reTyped.exec(s))) return { verb: 'type', text: m[1], label: m[2], into: !!m[2], submit: !!m[3] };
  if ((m = reSwipeIn.exec(s))) return { verb: 'swipe', dir: m[1], label: m[2], into: true };
  if ((m = reSwipeFrom.exec(s))) return { verb: 'swipefrom', x: m[1], y: m[2], x2: m[3], y2: m[4] };
  if ((m = reSwipe.exec(s))) return { verb: 'swipe', dir: m[1] };
  if ((m = rePressed.exec(s))) return { verb: 'press', key: m[1] };
  if ((m = reOpened.exec(s))) return { verb: 'open', app: m[1] };
  return null;
}

function parseAsked(s: string): Action | null {
  let m;
  if ((m = reCopyOne.exec(s))) return { verb: 'copy', label: m[1], phone: m[2] };
  if ((m = reCopyMany.exec(s))) return { verb: 'copymany', files: m[1], phone: m[2] };
  if (reListFolder.test(s)) return { verb: 'list' };
  if (reCopyFrom.test(s)) return { verb: 'copyfrom' };
  if (reCopyTo.test(s)) return { verb: 'copyto' };
  return null;
}

const ruleKeys: Record<string, string> = {
  Payments: 'rulePayments',
  Messages: 'ruleMessages',
  Commits: 'ruleCommits',
  'Installs and links': 'ruleInstallsLinks',
  Calls: 'ruleCalls',
  'Settings changes': 'ruleSettingsChanges',
  'Every action': 'ruleEveryAction',
  'Blocked by you': 'ruleBlockedByYou',
  credentials: 'ruleCredentials',
  'Read-only': 'ruleReadOnly',
  'Task apps': 'ruleTaskApps',
  'Agent scope': 'ruleAgentScope',
  'Finance apps': 'ruleFinanceApps',
  'Password managers': 'rulePasswordManagers',
  Files: 'ruleFiles',
  'Agent request': 'ruleAgentRequest',
};

const dirKeys: Record<string, string> = { up: 'gwDirUp', down: 'gwDirDown', left: 'gwDirLeft', right: 'gwDirRight' };

const keyKeys: Record<string, string> = {
  back: 'gwKeyBack',
  home: 'gwKeyHome',
  recents: 'gwKeyRecents',
  enter: 'gwKeyEnter',
  tab: 'gwKeyTab',
  delete: 'gwKeyDelete',
  'forward delete': 'gwKeyForwardDelete',
  escape: 'gwKeyEscape',
  'dpad up': 'gwKeyUp',
  'dpad down': 'gwKeyDown',
  'dpad left': 'gwKeyLeft',
  'dpad right': 'gwKeyRight',
  'page up': 'gwKeyPageUp',
  'page down': 'gwKeyPageDown',
};

const outcomeKeys: Record<string, string> = {
  done: 'gwOutcomeDone',
  failed: 'gwOutcomeFailed',
  partial: 'gwOutcomePartial',
  cancelled: 'gwOutcomeCancelled',
  stopped: 'gwOutcomeStopped',
  expired: 'gwOutcomeExpired',
  disconnected: 'gwOutcomeDisconnected',
  offline: 'gwOutcomeOffline',
  replaced: 'gwOutcomeReplaced',
};

function lowerFirst(s: string): string {
  if (s.length < 2) return s.toLowerCase();
  const [a, b] = [s[0], s[1]];
  if (a === a.toUpperCase() && a !== a.toLowerCase() && !(b === b.toUpperCase() && b !== b.toLowerCase())) {
    return a.toLowerCase() + s.slice(1);
  }
  return s;
}

/** "Tapped “Pay”" → "tap “Pay”", as the English approval steps word it. */
export function imperative(s: string): string {
  for (const [from, to] of [
    ['Long-pressed ', 'long-press '],
    ['Tapped ', 'tap '],
    ['Typed ', 'type '],
    ['Swiped ', 'swipe '],
    ['Pressed ', 'press '],
    ['Opened ', 'open '],
  ]) {
    if (s.startsWith(from)) return to + s.slice(from.length);
  }
  return lowerFirst(s);
}

export interface GwText {
  translated: boolean;
  stepText: (s: string) => string;
  stepSub: (s: string) => string;
  actionAsked: (past: string) => string;
  approvalText: (s: string) => string;
  needTitle: (n: Need) => string;
  needDetail: (n: Need) => string;
  statusLine: (d: Device) => string;
  ruleName: (rule: string) => string;
  scopeLine: (s: ScopeView) => string;
  joinAnd: (list: string[]) => string;
}

export function makeGwText(t: T, translated: boolean): GwText {
  const dirName = (d = '') => (dirKeys[d] ? t(dirKeys[d]) : d);
  const keyName = (k = '') => {
    const key = keyKeys[k.replaceAll('_', ' ')];
    return key ? t(key) : k;
  };
  const ruleName = (r: string) => (ruleKeys[r] ? t(ruleKeys[r]) : r);
  const joinAnd = (s: string[]) => {
    if (s.length === 0) return '';
    if (s.length === 1) return s[0];
    if (s.length === 2) return t('listPair', { first: s[0], second: s[1] });
    return t('listMany', { rest: s.slice(0, -1).join(t('listSeparator')), last: s[s.length - 1] });
  };
  const scopeLine = (s: ScopeView) => {
    switch (s.mode) {
      case 'apps':
        return s.apps.length === 0 ? t('scopeLineNoApp') : t('scopeLineApps', { apps: joinAnd(s.apps) });
      case 'read_only':
        return t('scopeLineReadOnly');
      case 'watch_only':
        return t('scopeLineWatchOnly');
    }
    return t('scopeLineEverything');
  };

  const done = (a: Action): string => {
    const long = a.verb === 'longpress';
    switch (a.verb) {
      case 'tap':
      case 'longpress':
        if (a.into) return t(long ? 'gwDidLongPress' : 'gwDidTap', { label: a.label });
        if (a.placed) return t(long ? 'gwDidLongPressAt' : 'gwDidTapAt', { x: a.x, y: a.y });
        return t(long ? 'gwDidLongPressBare' : 'gwDidTapBare');
      case 'type':
        if (a.into && a.submit) return t('gwDidTypeIntoSubmit', { text: a.text, label: a.label });
        if (a.into) return t('gwDidTypeInto', { text: a.text, label: a.label });
        if (a.submit) return t('gwDidTypeSubmit', { text: a.text });
        return t('gwDidType', { text: a.text });
      case 'swipe':
        if (a.into) return t('gwDidSwipeIn', { direction: dirName(a.dir), label: a.label });
        return t('gwDidSwipe', { direction: dirName(a.dir) });
      case 'swipefrom':
        return t('gwDidSwipeFromTo', { fromX: a.x, fromY: a.y, toX: a.x2, toY: a.y2 });
      case 'press':
        return t('gwDidPress', { key: keyName(a.key) });
      case 'open':
        return t('gwDidOpen', { app: a.app });
    }
    return ask(a);
  };

  const ask = (a: Action): string => {
    const long = a.verb === 'longpress';
    switch (a.verb) {
      case 'tap':
      case 'longpress':
        if (a.into) return t(long ? 'gwDoLongPress' : 'gwDoTap', { label: a.label });
        if (a.placed) return t(long ? 'gwDoLongPressAt' : 'gwDoTapAt', { x: a.x, y: a.y });
        return t(long ? 'gwDoLongPressBare' : 'gwDoTapBare');
      case 'type':
        if (a.into && a.submit) return t('gwDoTypeIntoSubmit', { text: a.text, label: a.label });
        if (a.into) return t('gwDoTypeInto', { text: a.text, label: a.label });
        if (a.submit) return t('gwDoTypeSubmit', { text: a.text });
        return t('gwDoType', { text: a.text });
      case 'swipe':
        if (a.into) return t('gwDoSwipeIn', { direction: dirName(a.dir), label: a.label });
        return t('gwDoSwipe', { direction: dirName(a.dir) });
      case 'swipefrom':
        return t('gwDoSwipeFromTo', { fromX: a.x, fromY: a.y, toX: a.x2, toY: a.y2 });
      case 'press':
        return t('gwDoPress', { key: keyName(a.key) });
      case 'open':
        return t('gwDoOpen', { app: a.app });
      case 'copy':
        return t('gwDoCopyFile', { path: a.label, phone: a.phone });
      case 'copymany':
        return t('gwDoCopyFiles', { count: Number(a.files), phone: a.phone });
      case 'list':
        return t('gwDoListFolder');
      case 'copyfrom':
        return t('gwDoCopyFromPhone');
      case 'copyto':
        return t('gwDoCopyToPhone');
    }
    return '';
  };

  const asked = (rest: string) => {
    const a = parseAction(rest) ?? parseAsked(rest);
    return a ? ask(a) : rest;
  };

  const fileNames = (s: string) => {
    if (!s.startsWith(': ')) return s;
    const parts = s
      .slice(2)
      .split(', ')
      .map((p) => {
        const m = /^and (\d+) more$/.exec(p);
        return m ? t('gwFilesMore', { count: Number(m[1]) }) : p;
      });
    return t('gwFilesList', { names: parts.join(t('listSeparator')) });
  };

  const stepText = (s: string): string => {
    if (!translated) return s;
    const a = parseAction(s);
    if (a) return done(a);
    switch (s) {
      case 'Unlocked from the computer':
        return t('gwStepUnlocked');
      case 'You took over':
        return t('gwStepYouTookOver');
      case 'You stopped the task':
        return t('gwStepYouStopped');
    }
    let m;
    if ((m = /^Waiting for you: (.*)$/.exec(s))) return t('gwStepWaiting', { action: asked(m[1]) });
    if ((m = /^Blocked by “(.*?)”: (.*)$/.exec(s))) return t('gwStepBlocked', { rule: ruleName(m[1]), action: asked(m[2]) });
    if ((m = /^Outside the scope: (.*)$/.exec(s))) return t('gwStepOutside', { action: asked(m[1]) });
    if ((m = /^Failed: (.*)$/.exec(s))) return t('gwStepFailed', { action: asked(m[1]) });
    if ((m = /^Connected to (.*) for “(.*)”$/.exec(s))) return t('gwStepConnected', { phone: m[1], task: m[2] });
    if ((m = /^Finished: (.*)$/.exec(s)))
      return t('gwStepFinished', { outcome: outcomeKeys[m[1]] ? t(outcomeKeys[m[1]]) : m[1] });
    if ((m = /^Waited for “(.*)”$/.exec(s))) return t('gwStepWaited', { text: m[1] });
    if ((m = /^Asked you to take over: (.*)$/.exec(s))) return t('gwStepAskedTakeOver', { reason: m[1] });
    if ((m = /^Asked you: (.*)$/.exec(s))) return t('gwStepAskedYou', { question: m[1] });
    if ((m = /^Listed the phone's Pinky folder \((\d+) files?\)$/.exec(s)))
      return t('gwStepListedFolder', { count: Number(m[1]) });
    if ((m = /^Copied (\d+) files? (from|to) the phone(: .*)?$/.exec(s)))
      return t(m[2] === 'from' ? 'gwStepCopiedFrom' : 'gwStepCopiedTo', {
        count: Number(m[1]),
        names: fileNames(m[3] ?? ''),
      });
    if ((m = /^You (allowed|denied): (.*)$/.exec(s)))
      return t(m[1] === 'allowed' ? 'gwStepYouAllowed' : 'gwStepYouDenied', { action: asked(m[2]) });
    return s;
  };

  const stepSub = (s: string): string => {
    if (!translated || !s) return s;
    const i = s.indexOf('\n');
    let first = i < 0 ? s : s.slice(0, i);
    let m;
    if ((m = /^Your note: (.*)$/.exec(first))) first = t('gwStepYourNote', { note: m[1] });
    else if ((m = /^Always allowed: “(.*?)” in (.+)$/.exec(first)))
      first = t('gwStepAlwaysAllowed', { rule: ruleName(m[1]), app: m[2] });
    return i < 0 ? first : first + '\n' + stepSub(s.slice(i + 1));
  };

  const actionAsked = (past: string) => {
    if (translated) {
      const a = parseAction(past) ?? parseAsked(past);
      if (a) return ask(a);
    }
    return imperative(past);
  };

  const approvalText = (s: string): string => {
    if (!translated) return s;
    if (s === 'Send the message?') return t('gwAskSendMessage');
    let m;
    if ((m = /^Pay: “(.*)”\?$/.exec(s))) return t('gwAskPay', { label: m[1] });
    if ((m = /^Send “(.*)”\?$/.exec(s))) return t('gwAskSend', { text: m[1] });
    if ((m = /^Change “(.*)”\?$/.exec(s))) return t('gwAskChange', { label: m[1] });
    if ((m = /^(Tap|Long-press|Type into|Swipe) “(.*)”\?$/.exec(s))) {
      const key = { Tap: 'gwAskTap', 'Long-press': 'gwAskLongPress', 'Type into': 'gwAskTypeInto', Swipe: 'gwAskSwipe' }[
        m[1]
      ]!;
      return t(key, { label: m[2] });
    }
    if ((m = /^Press (\S+) in “(.*)”\?$/.exec(s))) return t('gwAskPressIn', { key: keyName(m[1]), label: m[2] });
    if ((m = /^Let (.+) use your phones\?$/.exec(s))) return t('gwAskAgent', { agent: m[1] });
    if ((m = /^(Copy .+)\?$/.exec(s))) {
      const a = parseAsked(m[1]);
      if (a) return t('gwAskQuestion', { action: ask(a) });
    }
    if ((m = /^Open (.+)\?$/.exec(s)) && !m[1].includes('“')) return t('gwAskOpen', { app: m[1] });
    if ((m = /^(.+)\?$/.exec(s))) {
      const a = parseAction(m[1]);
      if (a) return t('gwAskQuestion', { action: ask(a) });
    }
    return s;
  };

  const needTitle = (n: Need): string => {
    if (!translated) return n.title;
    if (n.kind === 'unlock') {
      return n.title.startsWith('Unlock ') ? t('gwUnlockTitle', { phone: n.title.slice(7) }) : n.title;
    }
    if (n.kind === 'handoff' || n.rule === 'Agent request') return n.title;
    return approvalText(n.title);
  };

  const needDetail = (n: Need): string => {
    const d = n.detail ?? '';
    if (!translated) return d;
    let m;
    if (n.kind === 'unlock' && (m = /^(.+) is waiting for the phone to unlock · (.*)$/.exec(d)))
      return t('gwUnlockDetail', { agent: m[1], task: m[2] });
    if (
      n.kind === 'approval' &&
      n.rule === 'New agent' &&
      (m = /^(.+) wants to connect for “(.*)”\.(?: Allowing gives it (\S+) access\.)?$/.exec(d))
    ) {
      let out = t('gwAgentWants', { agent: m[1], task: m[2] });
      if (m[3]) out += t('gwAgentGives', { scope: scopeLine({ mode: m[3], apps: [] }) });
      return out;
    }
    if ((m = /^(.+) · into the phone's Pinky folder(: .*)?$/.exec(d)))
      return t('gwFilesDetail', { agent: m[1], names: fileNames(m[2] ?? '') });
    return d;
  };

  const statusLine = (dev: Device): string => {
    const s = dev.statusLine;
    if (!translated) return s;
    switch (s) {
      case 'Offline':
        return t('gwStatusOffline');
      case 'Locked':
        return t('gwStatusLocked');
      case 'Idle':
        return t('gwStatusIdle');
      case "You're in control":
        return t('gwStatusInControl');
      case 'Needs setup':
        return t('gwStatusNeedsSetup');
    }
    let m;
    if ((m = /^Offline · reconnecting for (.*)$/.exec(s))) return t('gwStatusReconnecting', { task: m[1] });
    if ((m = /^Offline · (.*)$/.exec(s))) return t('gwStatusOfflineDetail', { detail: m[1] });
    if ((m = /^Needs you · (.*)$/.exec(s))) return t('gwStatusNeedsYou', { title: m[1] });
    if ((m = /^Waiting for you · (.*)$/.exec(s))) return t('gwStatusWaiting', { title: approvalText(m[1]) });
    if ((m = /^Locked · unlock to continue (.*)$/.exec(s))) return t('gwStatusLockedTask', { task: m[1] });
    if ((m = /^You're in control · (.+) is paused$/.exec(s))) return t('gwStatusInControlPaused', { agent: m[1] });
    if ((m = /^(.*) · in the background$/.exec(s)) && dev.status === 'working')
      return t('gwStatusBackground', { work: m[1] });
    return s;
  };

  return {
    translated,
    stepText,
    stepSub,
    actionAsked,
    approvalText,
    needTitle,
    needDetail,
    statusLine,
    ruleName,
    scopeLine,
    joinAnd,
  };
}

/** The gateway's words in the language the client shows. */
export function useGw(): GwText {
  const { t, locale } = useI18n();
  return useMemo(() => makeGwText(t, locale !== 'en'), [t, locale]);
}
