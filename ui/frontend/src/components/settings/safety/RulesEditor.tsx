import { useEffect, useState } from 'react';
import { Alert, Box, Button, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import ArrowUpIcon from '@mui/icons-material/ArrowUpwardRounded';
import ArrowDownIcon from '@mui/icons-material/ArrowDownwardRounded';
import DeleteIcon from '@mui/icons-material/DeleteOutlineRounded';
import EditIcon from '@mui/icons-material/EditOutlined';
import AddIcon from '@mui/icons-material/AddRounded';
import { RowCard, Section, StatusText, useConfirm } from '@/components/common';
import { api, ApiError } from '@/api/client';
import { useT, type T } from '@/i18n';
import type { Rule, RulesView } from '@/api/types';
import { actionKeys, categoryKeys, joinAnd, ruleName } from '../labels';
import { errorText } from '../controls';
import { RuleDialog } from './RuleDialog';

const strictness: Record<string, number> = { allow: 0, ask: 1, deny: 2 };

function summary(t: T, r: Rule): string {
  const parts = [
    ...r.apps,
    ...r.categories.map((c) => t(categoryKeys[c] ?? c)),
    ...r.actions.map((a) => t(actionKeys[a] ?? a)),
    ...(r.labels.length ? [t('settingsRulesLabelsSummary', { labels: r.labels.join(', ') })] : []),
  ];
  return joinAnd(t, parts);
}

function DecisionChip({ d }: { d: string }) {
  const t = useT();
  const label = d === 'allow' ? t('settingsRulesAllow') : d === 'deny' ? t('settingsRulesDeny') : t('settingsRulesAsk');
  return <StatusText tone={d === 'deny' ? 'bad' : d === 'ask' ? 'warn' : 'good'}>{label}</StatusText>;
}

function RuleLine({ r, children }: { r: Rule; children?: React.ReactNode }) {
  const t = useT();
  return (
    <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 2, py: 1 }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="subtitle1">{ruleName(t, r.name)}</Typography>
        <Typography variant="body2" color="text.secondary" noWrap>
          {summary(t, r)}
        </Typography>
      </Box>
      <DecisionChip d={r.decision} />
      {children}
    </Stack>
  );
}

/** Settings › Safety › Rules: the rules file's rules, and Pinky's own. */
export function RulesEditor() {
  const t = useT();
  const [view, setView] = useState<RulesView | null>(null);
  const [err, setErr] = useState('');
  const [editing, setEditing] = useState<{ index: number; rule: Rule | null } | null>(null);
  const [confirm, confirmDialog] = useConfirm();
  const load = () => api.rules().then(setView, (e) => setErr(errorText(e)));
  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 2000);
    return () => window.clearInterval(id);
  }, []);
  if (!view) return err ? <Alert severity="error">{err}</Alert> : null;

  const save = async (next: Rule[]) => {
    try {
      setView(await api.setRules(view.file, next));
      setErr('');
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setErr(t('settingsRulesErrChanged'));
        void load();
      } else setErr(t('settingsRulesErrSave', { error: errorText(e) }));
    }
  };
  const move = (i: number, by: number) => {
    const next = [...view.file];
    [next[i], next[i + by]] = [next[i + by], next[i]];
    void save(next);
  };
  const remove = async (i: number) => {
    const r = view.file[i];
    const ok = await confirm({
      title: t('settingsRulesRemoveTitle', { rule: r.name }),
      body: r.decision !== 'allow' ? t('settingsRulesWeakenMessage') : undefined,
      confirm: t('settingsRulesRemoveConfirm'),
      cancel: t('settingsCancel'),
      danger: true,
    });
    if (ok) void save(view.file.filter((_, j) => j !== i));
  };
  const commit = async (r: Rule) => {
    if (!editing) return;
    const old = editing.rule;
    if (old && (strictness[r.decision] ?? 1) < (strictness[old.decision] ?? 1)) {
      const ok = await confirm({
        title: t('settingsRulesWeakenTitle', { rule: old.name }),
        body: t('settingsRulesWeakenMessage'),
        confirm: t('settingsSave'),
        cancel: t('settingsCancel'),
        danger: true,
      });
      if (!ok) return;
    }
    const next = [...view.file];
    if (editing.index < 0) next.push(r);
    else next[editing.index] = r;
    setEditing(null);
    void save(next);
  };

  return (
    <Section
      title={t('settingsRules')}
      description={t('settingsRulesIntro')}
      action={
        <Button variant="outlined" color="inherit" startIcon={<AddIcon />} onClick={() => setEditing({ index: -1, rule: null })}>
          {t('settingsRulesAdd')}
        </Button>
      }
    >
      {err && (
        <Alert severity="error" sx={{ mb: 1 }}>
          {err}
        </Alert>
      )}
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        {t('settingsRulesYours')}
      </Typography>
      {view.file.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {t('settingsRulesNone')}
        </Typography>
      ) : (
        <RowCard sx={{ mb: 2 }}>
          {view.file.map((r, i) => (
            <RuleLine key={r.name} r={r}>
              <Tooltip title={t('settingsRulesMoveUpTip')}>
                <span>
                  <IconButton disabled={i === 0} onClick={() => move(i, -1)} aria-label={t('settingsRulesMoveUp', { rule: r.name })}>
                    <ArrowUpIcon fontSize="small" />
                  </IconButton>
                </span>
              </Tooltip>
              <Tooltip title={t('settingsRulesMoveDownTip')}>
                <span>
                  <IconButton
                    disabled={i === view.file.length - 1}
                    onClick={() => move(i, 1)}
                    aria-label={t('settingsRulesMoveDown', { rule: r.name })}
                  >
                    <ArrowDownIcon fontSize="small" />
                  </IconButton>
                </span>
              </Tooltip>
              <Tooltip title={t('settingsChange')}>
                <IconButton onClick={() => setEditing({ index: i, rule: r })} aria-label={t('settingsRulesChangeLabel', { rule: r.name })}>
                  <EditIcon fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title={t('settingsRulesRemoveTip')}>
                <IconButton onClick={() => remove(i)} aria-label={t('settingsRulesRemove', { rule: r.name })}>
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </RuleLine>
          ))}
        </RowCard>
      )}
      <Typography variant="subtitle2">{t('settingsRulesBuiltIn')}</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
        {t('settingsRulesBuiltInDesc')}
      </Typography>
      <RowCard>
        {view.builtIn.map((r) => (
          <RuleLine key={r.name} r={r} />
        ))}
      </RowCard>
      {editing && (
        <RuleDialog
          rule={editing.rule}
          taken={view.file.map((r) => r.name)}
          onSave={commit}
          onClose={() => setEditing(null)}
        />
      )}
      {confirmDialog}
    </Section>
  );
}
