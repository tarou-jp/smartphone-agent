// Rows Settings is built from, on the common Section/RowCard/Row pieces.

import { useEffect, useState, type ReactNode } from 'react';
import { IconButton, MenuItem, Select, Stack, Switch, TextField, Tooltip, Typography, Box } from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopyRounded';
import VisibilityIcon from '@mui/icons-material/VisibilityOutlined';
import VisibilityOffIcon from '@mui/icons-material/VisibilityOffOutlined';
import { Row } from '@/components/common';
import { useApp } from '@/state/AppState';

export function SettingSwitchRow({
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  label: ReactNode;
  description?: ReactNode;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <Row
      label={label}
      description={description}
      control={
        <Switch
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
          inputProps={{ 'aria-label': typeof label === 'string' ? label : undefined }}
        />
      }
    />
  );
}

export function SettingSelectRow<V extends string | number>({
  label,
  description,
  value,
  options,
  onChange,
  minWidth = 160,
}: {
  label: ReactNode;
  description?: ReactNode;
  value: V;
  options: { value: V; label: ReactNode; disabled?: boolean }[];
  onChange: (v: V) => void;
  minWidth?: number;
}) {
  return (
    <Row
      label={label}
      description={description}
      control={
        <Select
          value={value}
          onChange={(e) => {
            const raw = e.target.value;
            const opt = options.find((o) => String(o.value) === String(raw));
            if (opt) onChange(opt.value);
          }}
          sx={{ minWidth, fontSize: 13 }}
        >
          {options.map((o) => (
            <MenuItem key={String(o.value)} value={o.value} disabled={o.disabled} sx={{ fontSize: 13 }}>
              {o.label}
            </MenuItem>
          ))}
        </Select>
      }
    />
  );
}

/** A text field that saves when it loses focus or Enter is pressed. */
export function CommitTextField({
  value,
  onCommit,
  validate,
  ...rest
}: {
  value: string;
  onCommit: (v: string) => void;
  validate?: (v: string) => string | null;
  label?: string;
  placeholder?: string;
  fullWidth?: boolean;
  sx?: object;
}) {
  const [draft, setDraft] = useState(value);
  const [err, setErr] = useState<string | null>(null);
  useEffect(() => setDraft(value), [value]);
  const commit = () => {
    if (draft === value) return;
    const e = validate?.(draft) ?? null;
    setErr(e);
    if (!e) onCommit(draft);
  };
  return (
    <TextField
      {...rest}
      value={draft}
      error={!!err}
      helperText={err ?? undefined}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') commit();
        if (e.key === 'Escape') {
          setDraft(value);
          setErr(null);
        }
      }}
    />
  );
}

/** A secret or command to copy, monospace, optionally hidden. */
export function CopyField({
  value,
  copyLabel,
  copiedToast,
  secret,
  showLabel,
  hideLabel,
}: {
  value: string;
  copyLabel: string;
  copiedToast: string;
  secret?: boolean;
  showLabel?: string;
  hideLabel?: string;
}) {
  const { toast } = useApp();
  const [shown, setShown] = useState(!secret);
  return (
    <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mt: 1 }}>
      <Box
        className="selectable"
        sx={{
          flex: 1,
          minWidth: 0,
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: 12,
          px: 1.25,
          py: 0.75,
          borderRadius: 1.5,
          bgcolor: (th) => th.pinky.bg2,
          border: 1,
          borderColor: 'divider',
          overflowX: 'auto',
          whiteSpace: 'nowrap',
        }}
      >
        <Typography component="span" sx={{ font: 'inherit' }}>
          {shown ? value : '•'.repeat(Math.min(value.length, 32))}
        </Typography>
      </Box>
      {secret && (
        <Tooltip title={shown ? hideLabel : showLabel}>
          <IconButton onClick={() => setShown(!shown)} aria-label={shown ? hideLabel : showLabel}>
            {shown ? <VisibilityOffIcon fontSize="small" /> : <VisibilityIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
      )}
      <Tooltip title={copyLabel}>
        <IconButton
          aria-label={copyLabel}
          onClick={() => {
            void navigator.clipboard?.writeText(value).then(() => toast(copiedToast, 'success'));
          }}
        >
          <ContentCopyIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </Stack>
  );
}

/** Calls fn every ms while mounted, and once at once. */
export function usePoll(fn: () => void, ms: number, on = true) {
  useEffect(() => {
    if (!on) return;
    fn();
    const id = window.setInterval(fn, ms);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ms, on]);
}

export function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}
