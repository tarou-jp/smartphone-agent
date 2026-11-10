// A small ICU MessageFormat formatter for the client's catalogs
// (cmd/pinky/locales/*.json): {name}, {n, plural, …} with # and =N,
// {x, select, …}, {d, date, …} and {d, time, …}, nested to any depth.

type Node =
  | string
  | { arg: string }
  | { arg: string; type: 'plural' | 'select'; options: Record<string, Node[]>; offset: number }
  | { arg: string; type: 'date' | 'time'; style: string }
  | { hash: true };

const cache = new Map<string, Node[]>();

function parse(src: string): Node[] {
  let i = 0;
  function parseMessage(inPlural: boolean): Node[] {
    const out: Node[] = [];
    let text = '';
    const flush = () => {
      if (text) out.push(text);
      text = '';
    };
    while (i < src.length) {
      const c = src[i];
      if (c === '}') break;
      if (c === "'" && (src[i + 1] === '{' || src[i + 1] === '}' || src[i + 1] === "'" || src[i + 1] === '#')) {
        // Quoted literal: '{' or '' etc.
        if (src[i + 1] === "'") {
          text += "'";
          i += 2;
          continue;
        }
        const end = src.indexOf("'", i + 1);
        text += end < 0 ? src.slice(i + 1) : src.slice(i + 1, end);
        i = end < 0 ? src.length : end + 1;
        continue;
      }
      if (c === '#' && inPlural) {
        flush();
        out.push({ hash: true });
        i++;
        continue;
      }
      if (c === '{') {
        flush();
        i++;
        out.push(parseArg());
        continue;
      }
      text += c;
      i++;
    }
    flush();
    return out;
  }
  function word(): string {
    while (src[i] === ' ' || src[i] === '\n') i++;
    let w = '';
    while (i < src.length && !/[\s,{}]/.test(src[i])) w += src[i++];
    while (src[i] === ' ' || src[i] === '\n') i++;
    return w;
  }
  function parseArg(): Node {
    const arg = word();
    if (src[i] === '}') {
      i++;
      return { arg };
    }
    i++; // ,
    const type = word();
    if (type === 'plural' || type === 'select' || type === 'selectordinal') {
      i++; // ,
      const options: Record<string, Node[]> = {};
      let offset = 0;
      for (;;) {
        const key = word();
        if (!key) break;
        if (key.startsWith('offset:')) {
          offset = Number(key.slice(7)) || 0;
          continue;
        }
        if (src[i] !== '{') break;
        i++;
        options[key] = parseMessage(type !== 'select');
        i++; // }
      }
      while (i < src.length && src[i] !== '}') i++;
      i++;
      return { arg, type: type === 'select' ? 'select' : 'plural', options, offset };
    }
    let style = '';
    if (src[i] === ',') {
      i++;
      style = word();
    }
    while (i < src.length && src[i] !== '}') i++;
    i++;
    return { arg, type: type === 'time' ? 'time' : 'date', style };
  }
  return parseMessage(false);
}

export type Params = Record<string, string | number | Date | undefined | null>;

export function format(src: string, params: Params | undefined, locale: string): string {
  let nodes = cache.get(src);
  if (!nodes) {
    nodes = parse(src);
    cache.set(src, nodes);
  }
  return render(nodes, params ?? {}, locale, undefined);
}

const plurals = new Map<string, Intl.PluralRules>();

function render(nodes: Node[], p: Params, locale: string, hash: number | undefined): string {
  let s = '';
  for (const n of nodes) {
    if (typeof n === 'string') {
      s += n;
    } else if ('hash' in n) {
      s += hash === undefined ? '#' : new Intl.NumberFormat(locale).format(hash);
    } else if (!('type' in n)) {
      const v = p[n.arg];
      s += v instanceof Date ? v.toLocaleString(locale) : v == null ? '' : String(v);
    } else if (n.type === 'plural') {
      const v = Number(p[n.arg] ?? 0);
      let opt = n.options['=' + v];
      if (!opt) {
        let pr = plurals.get(locale);
        if (!pr) {
          pr = new Intl.PluralRules(locale);
          plurals.set(locale, pr);
        }
        opt = n.options[pr.select(v - n.offset)] ?? n.options.other ?? [];
      }
      s += render(opt, p, locale, v - n.offset);
    } else if (n.type === 'select') {
      const v = String(p[n.arg] ?? '');
      s += render(n.options[v] ?? n.options.other ?? [], p, locale, hash);
    } else if (n.type === 'date' || n.type === 'time') {
      const raw = p[n.arg];
      const d = raw instanceof Date ? raw : new Date(raw as string | number);
      if (isNaN(d.getTime())) continue;
      s += d.toLocaleString(locale, skeleton(n.style, n.type));
    }
  }
  return s;
}

/** Intl options for an ICU date skeleton such as ::MMMMEEEEd or ::Hm. */
function skeleton(style: string, type: 'date' | 'time'): Intl.DateTimeFormatOptions {
  if (!style.startsWith('::')) {
    return type === 'time' ? { hour: 'numeric', minute: '2-digit' } : { month: 'short', day: 'numeric' };
  }
  const o: Intl.DateTimeFormatOptions = {};
  const sk = style.slice(2);
  const count = (ch: string) => (sk.match(new RegExp(ch + '+')) ?? [''])[0].length;
  if (count('y')) o.year = 'numeric';
  const m = count('M');
  if (m) o.month = m >= 4 ? 'long' : m === 3 ? 'short' : 'numeric';
  const e = count('E');
  if (e) o.weekday = e >= 4 ? 'long' : 'short';
  if (count('d')) o.day = 'numeric';
  if (count('j') || count('h')) o.hour = 'numeric';
  if (count('H')) {
    o.hour = '2-digit';
    o.hourCycle = 'h23';
  }
  if (count('m')) o.minute = '2-digit';
  return o;
}
