import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Alert, Box, Button, Collapse, IconButton, ListItemButton, Stack, Tooltip, Typography, useTheme } from '@mui/material';
import FlowsIcon from '@mui/icons-material/AccountTreeOutlined';
import DeleteIcon from '@mui/icons-material/DeleteOutlineRounded';
import ExpandIcon from '@mui/icons-material/ExpandMoreRounded';
import InfoIcon from '@mui/icons-material/InfoOutlined';
import { CardRows, EmptyState, PageHeader, Section, useConfirm } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { href, navigate } from '@/lib/router';
import { isRunning } from '@/lib/format';
import type { Library, LibraryApp, LibraryFlow, LibraryPart, LibraryStep } from '@/api/types';
import { AppIcon } from '@/components/flows/AppIcon';
import { NoteCards } from '@/components/flows/NoteCards';
import { PartShot } from '@/components/flows/PartShot';
import { splitNotes } from '@/lib/notes';

/** A small label beside a step: a part's id, or what is wrong with it. */
function Tag({ children, title, tone }: { children: ReactNode; title?: string; tone?: 'warn' | 'bad' }) {
  const p = useTheme().pinky;
  const color = tone === 'bad' ? p.red : tone === 'warn' ? p.amber : p.ink3;
  const tag = (
    <Box
      component="span"
      sx={{
        flexShrink: 0,
        fontFamily: tone ? undefined : p.mono,
        fontSize: 11.5,
        lineHeight: '18px',
        px: 0.75,
        borderRadius: '6px',
        border: 1,
        borderColor: tone === 'bad' ? p.redLine : tone === 'warn' ? p.amberLine : 'divider',
        bgcolor: tone === 'bad' ? p.redTint : tone === 'warn' ? p.amberTint : 'transparent',
        color,
        whiteSpace: 'nowrap',
      }}
    >
      {children}
    </Box>
  );
  return title ? <Tooltip title={title}>{tag}</Tooltip> : tag;
}

/** A section's title, with what it is about behind an info icon. */
function Title({ text, about }: { text: string; about: string }) {
  const p = useTheme().pinky;
  return (
    <Stack direction="row" alignItems="center" spacing={0.75}>
      <span>{text}</span>
      <Tooltip title={about} placement="right">
        <InfoIcon aria-label={about} sx={{ fontSize: 16, color: p.ink3, cursor: 'help' }} />
      </Tooltip>
    </Stack>
  );
}

/**
 * What one step does: why (the agent's own words, when it gave them), the
 * action as Pinky keeps it, and the screen it leads to.
 */
function StepText({ step }: { step: Pick<LibraryStep, 'text' | 'leads' | 'intent'> }) {
  const p = useTheme().pinky;
  return (
    <Box sx={{ flex: 1, minWidth: 0 }} className="selectable">
      {step.intent && <Typography sx={{ fontSize: 14, lineHeight: '20px' }}>{step.intent}</Typography>}
      <Typography sx={{ fontFamily: p.mono, fontSize: 12.5, lineHeight: '19px', color: step.intent ? p.ink2 : 'text.primary', overflowWrap: 'anywhere' }}>
        {step.text}
      </Typography>
      {step.leads && (
        <Typography sx={{ fontFamily: p.mono, fontSize: 12, lineHeight: '18px', color: p.ink3, overflowWrap: 'anywhere' }}>→ {step.leads}</Typography>
      )}
    </Box>
  );
}

function FlowCard({ flow, onRemove }: { flow: LibraryFlow; onRemove: () => void }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const title = flow.description || flow.task || flow.name;
  const shots = flow.steps.filter((s) => s.part && s.shot);
  return (
    <CardRows sx={{ mb: 1.5, '& > *:not(:last-child)': { borderBottom: 0 } }}>
      <Stack direction="row" alignItems="center" spacing={0.5} sx={{ pl: 2, pr: 1, py: 1.25 }}>
        <Box
          role="button"
          tabIndex={0}
          aria-expanded={open}
          aria-label={open ? t('flowsHideSteps') : t('flowsShowSteps')}
          onClick={() => setOpen((v) => !v)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setOpen((v) => !v);
            }
          }}
          sx={{ flex: 1, minWidth: 0, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 1 }}
        >
          <ExpandIcon sx={{ fontSize: 20, color: p.ink3, flexShrink: 0, transform: open ? 'none' : 'rotate(-90deg)', transition: 'transform 120ms' }} />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="subtitle1" noWrap={!open} sx={{ overflowWrap: 'anywhere' }}>
              {title}
            </Typography>
            <Typography noWrap sx={{ fontSize: 12.5, color: p.ink3 }}>
              {t('flowsFlowSteps', { steps: flow.steps.length })}
            </Typography>
          </Box>
        </Box>
        <Tooltip title={t('flowsRemove')}>
          <IconButton aria-label={t('flowsRemove')} onClick={onRemove} sx={{ color: p.red, '&:hover': { bgcolor: p.redTint } }}>
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>
      {!open && shots.length > 0 && (
        <Stack direction="row" spacing={1} sx={{ px: 2, pb: 1.5, pt: 0.25, overflowX: 'auto' }}>
          {shots.map((s, i) => (
            <Box key={i} sx={{ position: 'relative' }}>
              <PartShot part={s.part ?? ''} shot width={64} caption={s.intent || s.text} />
              <Box
                sx={{
                  position: 'absolute',
                  left: 4,
                  top: 4,
                  minWidth: 16,
                  height: 16,
                  px: 0.5,
                  borderRadius: '8px',
                  bgcolor: 'rgba(0,0,0,.6)',
                  color: '#fff',
                  fontFamily: p.mono,
                  fontSize: 10.5,
                  lineHeight: '16px',
                  textAlign: 'center',
                  pointerEvents: 'none',
                }}
              >
                {flow.steps.indexOf(s) + 1}
              </Box>
            </Box>
          ))}
        </Stack>
      )}
      <Collapse in={open} unmountOnExit>
        <Box component="ol" sx={{ m: 0, p: 0, listStyle: 'none', borderTop: 1, borderColor: 'divider', '& > li:not(:last-child)': { borderBottom: 1, borderColor: 'divider' } }}>
          {flow.steps.map((s, i) => (
            <Stack component="li" key={i} direction="row" alignItems="flex-start" spacing={1.5} sx={{ px: 2, py: 1.25 }}>
              <Typography sx={{ width: 22, flexShrink: 0, textAlign: 'right', fontFamily: p.mono, fontSize: 12, lineHeight: '20px', color: p.ink3 }}>{i + 1}</Typography>
              <StepText step={s} />
              {s.part && s.shot && <PartShot part={s.part} shot width={44} caption={s.intent || s.text} />}
              <Stack direction="row" spacing={0.75} sx={{ pt: '1px' }}>
                {s.optional && <Tag>{t('flowsOptional')}</Tag>}
                {s.missing ? (
                  <Tag tone="bad" title={t('flowsPartMissingAbout')}>
                    {t('flowsPartMissing')}
                  </Tag>
                ) : (
                  s.part && <Tag title={t('flowsPartTag')}>{s.part}</Tag>
                )}
              </Stack>
            </Stack>
          ))}
        </Box>
      </Collapse>
    </CardRows>
  );
}

/** One labeled fact in an open part. */
function Fact({ label, children, mono }: { label: string; children: ReactNode; mono?: boolean }) {
  const p = useTheme().pinky;
  return (
    <Stack direction="row" spacing={1.5} sx={{ py: 0.5 }}>
      <Typography sx={{ width: 112, flexShrink: 0, fontSize: 12.5, lineHeight: '20px', color: p.ink3 }}>{label}</Typography>
      <Typography
        className="selectable"
        component="div"
        sx={{ flex: 1, minWidth: 0, fontSize: mono ? 12.5 : 13.5, lineHeight: '20px', fontFamily: mono ? p.mono : undefined, overflowWrap: 'anywhere' }}
      >
        {children}
      </Typography>
    </Stack>
  );
}

/**
 * A part as a row: what it is for and how often it worked; opened, the
 * picture of where it leads and everything Pinky keeps of it.
 */
function PartRow({ part, onRemove }: { part: LibraryPart; onRemove: () => void }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const title = part.intent || part.text;
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
        <Typography sx={{ flex: 1, minWidth: 0, fontSize: 14, lineHeight: '20px', opacity: part.broken ? 0.7 : 1 }} noWrap={!open}>
          {title}
        </Typography>
        {part.broken && (
          <Tag tone="warn" title={t('flowsPartBrokenAbout')}>
            {t('flowsPartBroken')}
          </Tag>
        )}
        {(part.flows?.length ?? 0) > 0 && <Tag title={t('flowsPartTag')}>{t('flowsPartInFlows', { count: part.flows!.length })}</Tag>}
        <Typography component="span" sx={{ fontSize: 12.5, color: p.ink3, whiteSpace: 'nowrap', minWidth: 56, textAlign: 'right' }}>
          {t('flowsPartCount', { count: part.seen })}
        </Typography>
      </Stack>
      <Collapse in={open} unmountOnExit>
        <Stack direction="row" spacing={2} sx={{ pl: 5.25, pr: 2, pb: 1.75, pt: 0.5 }}>
          <PartShot part={part.id} shot={part.shot} width={110} caption={title} />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            {part.intent && <Fact label={t('flowsFactPurpose')}>{part.intent}</Fact>}
            <Fact label={t('flowsFactAction')} mono>
              {part.text}
            </Fact>
            {part.leads && (
              <Fact label={t('flowsFactLeads')} mono>
                {part.leads}
              </Fact>
            )}
            <Fact label={t('flowsFactSeen')}>{t('flowsPartSeen', { count: part.seen, last: part.last ?? '' })}</Fact>
            {(part.flows?.length ?? 0) > 0 && <Fact label={t('flowsFactFlows')}>{part.flows!.join(', ')}</Fact>}
            {part.broken && <Fact label={t('flowsPartBroken')}>{t('flowsPartBrokenAbout')}</Fact>}
            <Fact label="ID" mono>
              {part.id}
            </Fact>
            <Box sx={{ mt: 1 }}>
              <Button size="small" variant="outlined" startIcon={<DeleteIcon sx={{ fontSize: 18 }} />} onClick={onRemove} sx={{ color: p.red, borderColor: p.redLine, '&:hover': { bgcolor: p.redTint, borderColor: p.red } }}>
                {t('flowsRemove')}
              </Button>
            </Box>
          </Box>
        </Stack>
      </Collapse>
    </Box>
  );
}

function AppDetail({ lib, app, onChange }: { lib: Library; app: LibraryApp; onChange: (l: Library) => void }) {
  const t = useT();
  const p = useTheme().pinky;
  const { run, toast } = useApp();
  const [confirm, dialog] = useConfirm();

  const saveNotes = async (text: string) => {
    const next = await run(t('flowsCallSaveNotes'), () => api.setNotes(app.app, text));
    if (!next) return false;
    onChange(next);
    toast(t('flowsNotesSaved'), 'success');
    return true;
  };
  const removeFlow = async (f: LibraryFlow) => {
    const yes = await confirm({
      title: t('flowsRemoveFlowTitle'),
      body: t('flowsRemoveFlowBody', { name: f.description || f.name }),
      confirm: t('flowsRemove'),
      cancel: t('settingsCancel'),
      danger: true,
    });
    if (!yes) return;
    const next = await run(t('flowsCallRemoveFlow'), () => api.removeFlow(f.name));
    if (next) onChange(next);
  };
  const removePart = async (part: LibraryPart) => {
    const used = part.flows ?? [];
    const yes = await confirm({
      title: t('flowsRemovePartTitle'),
      body: (
        <>
          <Box sx={{ fontFamily: p.mono, fontSize: 12.5, mb: 1, overflowWrap: 'anywhere' }}>{part.text}</Box>
          {t('flowsRemovePartBody')}
          {used.length > 0 && <Box sx={{ mt: 1, color: p.amber }}>{t('flowsRemovePartUsed', { count: used.length, names: used.join(', ') })}</Box>}
        </>
      ),
      confirm: t('flowsRemove'),
      cancel: t('settingsCancel'),
      danger: true,
    });
    if (!yes) return;
    const next = await run(t('flowsCallRemovePart'), () => api.removePart(part.id));
    if (next) onChange(next);
  };

  return (
    <Box sx={{ flex: 1, minWidth: 0, overflowY: 'auto', scrollbarGutter: 'stable' }}>
      <Box sx={{ maxWidth: 880, mx: 'auto', px: 5, pt: 3, pb: 6 }}>
        <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 4 }}>
          <AppIcon app={app.app} size={52} />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="h1" className="selectable" sx={{ fontSize: 22, overflowWrap: 'anywhere' }}>
              {app.app || t('flowsNoApp')}
            </Typography>
            <Typography sx={{ fontSize: 13, color: p.ink3, mt: 0.25 }}>
              {t('flowsAppSummary', { flows: app.flows.length, parts: app.parts.length })}
            </Typography>
          </Box>
        </Stack>
        {app.app && (
          <Section title={<Title text={t('flowsNotes')} about={t('flowsNotesAbout')} />}>
            <NoteCards key={app.app} saved={app.notes} max={lib.maxNotes} onSave={saveNotes} />
          </Section>
        )}
        <Section title={<Title text={t('flowsFlows')} about={t('flowsFlowsAbout')} />}>
          {app.flows.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              {t('flowsNoFlows')}
            </Typography>
          ) : (
            app.flows.map((f) => <FlowCard key={f.name} flow={f} onRemove={() => void removeFlow(f)} />)
          )}
        </Section>
        {app.app && (
          <Section title={<Title text={t('flowsParts')} about={t('flowsPartsAbout')} />} sx={{ mb: 0 }}>
            {app.parts.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                {t('flowsNoParts')}
              </Typography>
            ) : (
              <CardRows>
                {app.parts.map((part) => (
                  <PartRow key={part.id} part={part} onRemove={() => void removePart(part)} />
                ))}
              </CardRows>
            )}
          </Section>
        )}
      </Box>
      {dialog}
    </Box>
  );
}

/** Flows: what Pinky keeps of each app to replay tasks in it. */
export function FlowsPage({ app }: { app?: string }) {
  const t = useT();
  const p = useTheme().pinky;
  const { live } = useApp();
  const [lib, setLib] = useState<Library | null>(null);
  const [err, setErr] = useState('');
  const abort = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    abort.current?.abort();
    const ac = new AbortController();
    abort.current = ac;
    try {
      const l = await api.library(ac.signal);
      setLib(l);
      setErr('');
    } catch (e) {
      if (!ac.signal.aborted) setErr(t('flowsReadFailed', { detail: e instanceof Error ? e.message : String(e) }));
    }
  }, [t]);

  // A task that ends leaves flows and parts, and a step may leave a note.
  const running = live.tasks.filter(isRunning);
  const pulse = `${live.tasks.length - running.length}/${running.reduce((n, x) => n + x.steps.length, 0)}`;
  useEffect(() => {
    void load();
  }, [load, pulse]);

  const apps = lib?.apps ?? [];
  const cur = apps.find((a) => a.app === (app ?? '')) ?? (app === undefined ? apps[0] : undefined);
  // An address that names an app Pinky keeps nothing of (the last of it
  // was just removed) falls back to the first.
  useEffect(() => {
    if (lib && app !== undefined && !cur) navigate({ page: 'flows' }, true);
  }, [lib, app, cur]);

  return (
    <>
      <PageHeader title={t('flowsTitle')} />
      {err && (
        <Alert severity="error" sx={{ borderRadius: 0 }}>
          {err}
        </Alert>
      )}
      {lib?.broken && lib.broken.length > 0 && (
        <Alert severity="warning" sx={{ borderRadius: 0 }}>
          {t('flowsBrokenFiles', { names: lib.broken.join(', ') })}
        </Alert>
      )}
      {lib && apps.length === 0 ? (
        <EmptyState icon={<FlowsIcon />} title={t('flowsEmptyTitle')}>
          {t('flowsEmptyBody')}
        </EmptyState>
      ) : (
        <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <Box
            component="nav"
            aria-label={t('flowsAppsLabel')}
            sx={{ width: 288, flexShrink: 0, borderRight: 1, borderColor: 'divider', px: 1.5, pt: 2, pb: 2, overflowY: 'auto' }}
          >
            {apps.map((a) => {
              const on = a === cur;
              return (
                <ListItemButton
                  key={a.app}
                  component="a"
                  href={href({ page: 'flows', app: a.app || undefined })}
                  selected={on}
                  sx={{ py: 1, px: 1.25, mb: 0.25, gap: 1.25, alignItems: 'center' }}
                >
                  <AppIcon app={a.app} size={34} />
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography noWrap sx={{ fontSize: 14, fontWeight: on ? 500 : 400 }}>
                      {a.app || t('flowsNoApp')}
                    </Typography>
                    <Typography noWrap sx={{ fontSize: 12.5, color: p.ink3 }}>
                      {t('flowsAppSummary', { flows: a.flows.length, parts: a.parts.length })}
                      {splitNotes(a.notes).length > 0 && ` · ${t('flowsNotes')} ${splitNotes(a.notes).length}`}
                    </Typography>
                  </Box>
                </ListItemButton>
              );
            })}
          </Box>
          {cur && lib && <AppDetail key={cur.app} lib={lib} app={cur} onChange={setLib} />}
        </Box>
      )}
    </>
  );
}
