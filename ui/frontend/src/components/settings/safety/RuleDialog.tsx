import { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useT } from '@/i18n';
import type { Rule } from '@/api/types';
import { actionKeys, actions, categories, categoryKeys, splitList } from '../labels';

function ChipPicker({ options, label, value, onChange }: { options: string[]; label: (k: string) => string; value: string[]; onChange: (v: string[]) => void }) {
  return (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
      {options.map((o) => {
        const on = value.includes(o);
        return (
          <Chip
            key={o}
            label={label(o)}
            color={on ? 'primary' : 'default'}
            variant={on ? 'filled' : 'outlined'}
            onClick={() => onChange(on ? value.filter((x) => x !== o) : [...value, o])}
          />
        );
      })}
    </Box>
  );
}

/** Adds or changes one of the person's rules. */
export function RuleDialog({
  rule,
  taken,
  onSave,
  onClose,
}: {
  rule: Rule | null;
  taken: string[];
  onSave: (r: Rule) => void;
  onClose: () => void;
}) {
  const t = useT();
  const [r, setR] = useState<Rule>(
    rule ?? { name: '', apps: [], categories: [], actions: [], labels: [], decision: 'ask' },
  );
  const [apps, setApps] = useState((rule?.apps ?? []).join(', '));
  const [labels, setLabels] = useState((rule?.labels ?? []).join(', '));
  const [err, setErr] = useState('');
  const save = () => {
    const next: Rule = { ...r, name: r.name.trim(), apps: splitList(apps), labels: splitList(labels) };
    if (!next.name) return setErr(t('settingsRulesErrName'));
    if (taken.includes(next.name) && next.name !== rule?.name) return setErr(t('settingsRulesErrNameTaken', { rule: next.name }));
    if (!next.apps.length && !next.categories.length && !next.actions.length && !next.labels.length)
      return setErr(t('settingsRulesErrMatch'));
    onSave(next);
  };
  const field = (title: string, node: React.ReactNode) => (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 0.75 }}>
        {title}
      </Typography>
      {node}
    </Box>
  );
  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 15, fontWeight: 650 }}>
        {rule ? t('settingsRulesChangeLabel', { rule: rule.name }) : t('settingsRulesAdd')}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {t('settingsRulesHowItMatches')}
        </Typography>
        <TextField
          fullWidth
          autoFocus
          label={t('settingsRulesName')}
          placeholder={t('settingsRulesNamePlaceholder')}
          value={r.name}
          onChange={(e) => setR({ ...r, name: e.target.value })}
          sx={{ mb: 2 }}
        />
        {field(
          t('settingsRulesApps'),
          <TextField fullWidth placeholder={t('settingsRulesAppsPlaceholder')} value={apps} onChange={(e) => setApps(e.target.value)} />,
        )}
        {field(
          t('settingsRulesCategories'),
          <ChipPicker options={categories} label={(k) => t(categoryKeys[k] ?? k)} value={r.categories} onChange={(v) => setR({ ...r, categories: v })} />,
        )}
        {field(
          t('settingsRulesActions'),
          <ChipPicker options={actions} label={(k) => t(actionKeys[k] ?? k)} value={r.actions} onChange={(v) => setR({ ...r, actions: v })} />,
        )}
        {field(
          t('settingsRulesLabels'),
          <TextField fullWidth placeholder={t('settingsRulesLabelsPlaceholder')} value={labels} onChange={(e) => setLabels(e.target.value)} />,
        )}
        {field(
          t('settingsRulesDecision'),
          <RadioGroup row value={r.decision} onChange={(e) => setR({ ...r, decision: e.target.value })}>
            <FormControlLabel value="allow" control={<Radio size="small" />} label={t('settingsRulesAllow')} />
            <FormControlLabel value="ask" control={<Radio size="small" />} label={t('settingsRulesAsk')} />
            <FormControlLabel value="deny" control={<Radio size="small" />} label={t('settingsRulesDeny')} />
          </RadioGroup>,
        )}
        {r.decision === 'allow' && (
          <Typography variant="body2" color="text.secondary">
            {t('settingsRulesAllowHint')}
          </Typography>
        )}
        {err && (
          <Alert severity="error" sx={{ mt: 1 }}>
            {err}
          </Alert>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Stack direction="row" spacing={1}>
          <Button color="inherit" onClick={onClose}>
            {t('settingsCancel')}
          </Button>
          <Button variant="contained" onClick={save}>
            {t('settingsSave')}
          </Button>
        </Stack>
      </DialogActions>
    </Dialog>
  );
}
