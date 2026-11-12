import { useState } from 'react';
import { Box, Button, Collapse, LinearProgress, Stack, TextField, Typography, useTheme } from '@mui/material';
import AddIcon from '@mui/icons-material/AddRounded';
import EditIcon from '@mui/icons-material/EditOutlined';
import DeleteIcon from '@mui/icons-material/DeleteOutlineRounded';
import ExpandIcon from '@mui/icons-material/ExpandMoreRounded';
import { CardRows, useConfirm } from '@/components/common';
import { useT } from '@/i18n';
import { joinNotes, splitNotes } from '@/lib/notes';

const bytes = (s: string) => new TextEncoder().encode(s).length;

/** A note: its first line is the title, the lines under it the description. */
type Note = { title: string; body: string };

const toNote = (text: string): Note => {
  const [title, ...rest] = text.split('\n');
  return { title: title.trim(), body: rest.join('\n').trim() };
};

const fromNote = (n: Note): string => {
  const title = n.title.replace(/\s+/g, ' ').trim();
  // A blank line would end the note in the file, so the description keeps none.
  const body = n.body
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .join('\n');
  return body ? `${title}\n${body}` : title;
};

/** A note being written: an existing one by index, or a new one. */
type Editing = { at: number | 'new'; draft: Note } | null;

function Editor({
  value,
  onChange,
  onSave,
  onCancel,
  busy,
  error,
}: {
  value: Note;
  onChange: (v: Note) => void;
  onSave: () => void;
  onCancel: () => void;
  busy: boolean;
  error?: string;
}) {
  const t = useT();
  const keys = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      onSave();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      onCancel();
    }
  };
  const field = { '& .MuiInputBase-root': { fontSize: 14, lineHeight: '21px', bgcolor: 'background.paper' } };
  return (
    <Stack spacing={1.25}>
      <TextField
        value={value.title}
        onChange={(e) => onChange({ ...value, title: e.target.value.replace(/\n/g, ' ') })}
        autoFocus
        fullWidth
        size="small"
        label={t('flowsNoteTitle')}
        placeholder={t('flowsNoteTitlePlaceholder')}
        inputProps={{ spellCheck: false }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !(e.metaKey || e.ctrlKey)) e.preventDefault();
          keys(e);
        }}
        sx={field}
      />
      <TextField
        value={value.body}
        onChange={(e) => onChange({ ...value, body: e.target.value })}
        multiline
        minRows={3}
        maxRows={12}
        fullWidth
        label={t('flowsNoteBody')}
        placeholder={t('flowsNoteBodyPlaceholder')}
        error={!!error}
        helperText={error}
        inputProps={{ spellCheck: false }}
        onKeyDown={keys}
        sx={field}
      />
      <Stack direction="row" spacing={1} justifyContent="flex-end">
        <Button size="small" color="inherit" disabled={busy} onClick={onCancel}>
          {t('flowsNotesCancel')}
        </Button>
        <Button size="small" variant="contained" disabled={busy || !value.title.trim() || !!error} onClick={onSave}>
          {t('flowsNotesSave')}
        </Button>
      </Stack>
    </Stack>
  );
}

/** A note as a row: its title; opened, its description and what to do with it. */
function NoteRow({ n, i, busy, onEdit, onRemove }: { n: Note; i: number; busy: boolean; onEdit: () => void; onRemove: () => void }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const toggle = () => setOpen((v) => !v);
  return (
    <Box>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.25}
        role="button"
        tabIndex={0}
        aria-expanded={open}
        onClick={toggle}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            toggle();
          }
        }}
        sx={{ pl: 1.25, pr: 2, py: 1, cursor: 'pointer', '&:hover': { bgcolor: p.bg2 } }}
      >
        <ExpandIcon sx={{ fontSize: 20, color: p.ink3, flexShrink: 0, transform: open ? 'none' : 'rotate(-90deg)', transition: 'transform 120ms' }} />
        <Typography sx={{ fontFamily: p.mono, fontSize: 11.5, color: p.ink3, flexShrink: 0, minWidth: 20 }}>#{i + 1}</Typography>
        <Typography sx={{ flex: 1, minWidth: 0, fontSize: 14, lineHeight: '20px' }} noWrap={!open}>
          {n.title}
        </Typography>
      </Stack>
      <Collapse in={open} unmountOnExit>
        <Box sx={{ pl: 5.25, pr: 2, pb: 1.75, pt: 0.25 }}>
          {n.body ? (
            <Typography className="selectable" sx={{ fontSize: 13.5, lineHeight: '21px', whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
              {n.body}
            </Typography>
          ) : (
            <Typography sx={{ fontSize: 13, color: p.ink3 }}>{t('flowsNoteNoBody')}</Typography>
          )}
          <Stack direction="row" spacing={1} sx={{ mt: 1.25 }}>
            <Button size="small" variant="outlined" color="inherit" startIcon={<EditIcon sx={{ fontSize: 17 }} />} disabled={busy} onClick={onEdit} sx={{ borderColor: p.line2 }}>
              {t('flowsNotesEdit')}
            </Button>
            <Button
              size="small"
              variant="outlined"
              startIcon={<DeleteIcon sx={{ fontSize: 18 }} />}
              disabled={busy}
              onClick={onRemove}
              sx={{ color: p.red, borderColor: p.redLine, '&:hover': { bgcolor: p.redTint, borderColor: p.red } }}
            >
              {t('flowsRemove')}
            </Button>
          </Stack>
        </Box>
      </Collapse>
    </Box>
  );
}

/**
 * The notes on an app, one row each with its title; a row opens to its
 * description. Agents add one note at a time, and the person adds,
 * rewrites and removes them one by one. Each change saves the whole list.
 */
export function NoteCards({ saved, max, onSave }: { saved: string; max: number; onSave: (text: string) => Promise<boolean> }) {
  const t = useT();
  const p = useTheme().pinky;
  const [confirm, dialog] = useConfirm();
  const [editing, setEditing] = useState<Editing>(null);
  const [busy, setBusy] = useState(false);
  const notes = splitNotes(saved).map(toNote);

  // What the notes would be with the note being written.
  const proposed = (e: NonNullable<Editing>) => {
    const next = notes.map(fromNote);
    if (e.at === 'new') next.push(fromNote(e.draft));
    else next[e.at] = fromNote(e.draft);
    return joinNotes(next);
  };
  const tooLong = editing ? bytes(proposed(editing)) > max : false;
  const pct = Math.min(100, Math.round((bytes(saved) / max) * 100));

  const commit = async (text: string) => {
    setBusy(true);
    const ok = await onSave(text);
    setBusy(false);
    return ok;
  };
  const save = async () => {
    if (!editing || !editing.draft.title.trim() || tooLong) return;
    if (await commit(proposed(editing))) setEditing(null);
  };
  const remove = async (i: number) => {
    const yes = await confirm({
      title: t('flowsRemoveNoteTitle'),
      body: (
        <>
          <Box sx={{ mb: 1, fontWeight: 600, overflowWrap: 'anywhere' }}>{notes[i].title}</Box>
          {t('flowsRemoveNoteBody')}
        </>
      ),
      confirm: t('flowsRemove'),
      cancel: t('settingsCancel'),
      danger: true,
    });
    if (!yes) return;
    await commit(joinNotes(notes.filter((_, j) => j !== i).map(fromNote)));
  };

  const editor = (at: number | 'new', draft: Note) => (
    <Box sx={{ p: 1.5, bgcolor: p.bg2 }}>
      <Editor
        value={draft}
        onChange={(d) => setEditing({ at, draft: d })}
        onSave={() => void save()}
        onCancel={() => setEditing(null)}
        busy={busy}
        error={tooLong ? t('flowsNotesTooLong') : undefined}
      />
    </Box>
  );

  return (
    <Box>
      {(notes.length > 0 || editing?.at === 'new') && (
        <CardRows sx={{ mb: 1.25 }}>
          {notes.map((n, i) =>
            editing?.at === i ? (
              <Box key={i}>{editor(i, editing.draft)}</Box>
            ) : (
              <NoteRow key={i} n={n} i={i} busy={busy} onEdit={() => setEditing({ at: i, draft: n })} onRemove={() => void remove(i)} />
            ),
          )}
          {editing?.at === 'new' && <Box>{editor('new', editing.draft)}</Box>}
        </CardRows>
      )}
      <Stack direction="row" alignItems="center" spacing={2}>
        {editing?.at !== 'new' && (
          <Button size="small" variant="outlined" color="inherit" startIcon={<AddIcon />} disabled={busy} onClick={() => setEditing({ at: 'new', draft: { title: '', body: '' } })} sx={{ borderColor: p.line2 }}>
            {t('flowsNotesAdd')}
          </Button>
        )}
        {pct >= 50 && (
          <Stack direction="row" alignItems="center" spacing={1.25}>
            <LinearProgress
              variant="determinate"
              value={pct}
              color={pct >= 90 ? 'warning' : 'inherit'}
              sx={{ width: 96, height: 4, borderRadius: 2, color: pct >= 90 ? undefined : p.ink3, bgcolor: p.bg3 }}
            />
            <Typography sx={{ fontSize: 12, color: p.ink3 }}>{t('flowsNotesUsage', { pct })}</Typography>
          </Stack>
        )}
      </Stack>
      {dialog}
    </Box>
  );
}
