import { Alert, Box, Typography } from '@mui/material';
import { Row, RowCard, Section } from '@/components/common';
import { useApp } from '@/state/AppState';
import { useT, type T } from '@/i18n';
import { CommitTextField, CopyField, SettingSwitchRow } from '../controls';

/** Checks host:port as ui/remote.go does, returning the problem or null. */
export function remoteAddrError(t: T, addr: string): string | null {
  const m = addr.trim().match(/^(\[[^\]]+\]|[^:]+):(\d+)$/);
  if (!m) return t('remoteAddrInvalid');
  const port = Number(m[2]);
  if (!(port >= 1 && port <= 65535)) return t('remotePortInvalid');
  const host = m[1].replace(/^\[|\]$/g, '');
  if (host !== 'localhost' && !/^[\d.]+$/.test(host) && !host.includes(':')) return t('remoteHostInvalid');
  return null;
}

const local = (addr: string) => /^(127\.|localhost|\[::1\])/.test(addr);

/** Remote agents: MCP over HTTP with the agent token. */
export function RemoteAgents() {
  const t = useT();
  const { settings, patchSettings, boot } = useApp();
  if (!settings) return null;
  const token = boot?.agentToken ?? '';
  const url = `http://${settings.remoteAddr}/mcp`;
  const cmd = `claude mcp add --transport http pinky ${url} --header "Authorization: Bearer ${token}"`;
  return (
    <Section title={t('remoteTitle')} description={t('remoteIntro')}>
      <RowCard>
        <SettingSwitchRow
          label={t('remoteToggle')}
          checked={settings.remoteAgents}
          onChange={(v) => patchSettings({ remoteAgents: v })}
        />
        {settings.remoteAgents && (
          <Row label={t('remoteAddress')} description={t('remoteAddressDesc')}>
            <CommitTextField
              value={settings.remoteAddr}
              validate={(v) => remoteAddrError(t, v)}
              onCommit={(v) => patchSettings({ remoteAddr: v.trim() })}
              sx={{ mt: 1, width: 260 }}
            />
            {!local(settings.remoteAddr) && (
              <Alert severity="warning" sx={{ mt: 1 }}>
                {t('remoteOthersWarning')}
              </Alert>
            )}
          </Row>
        )}
        {settings.remoteAgents && (
          <Row label={t('remoteURLTitle')}>
            <CopyField value={url} copyLabel={t('remoteCopyURL')} copiedToast={t('remoteCopiedURL')} />
          </Row>
        )}
        <Row label={t('remoteToken')} description={t('remoteTokenDesc')}>
          {token ? (
            <CopyField
              value={token}
              secret
              copyLabel={t('remoteCopyToken')}
              copiedToast={t('remoteCopiedToken')}
              showLabel={t('remoteShowToken')}
              hideLabel={t('remoteHideToken')}
            />
          ) : (
            <Typography variant="body2" color="text.secondary">
              {t('remoteTokenMissing')}
            </Typography>
          )}
        </Row>
        {settings.remoteAgents && token && (
          <Row label={t('remoteCommandLabel')} description={t('remoteRunHint')}>
            <Box>
              <CopyField value={cmd} copyLabel={t('remoteCopyCommand')} copiedToast={t('remoteCopiedCommand')} />
            </Box>
          </Row>
        )}
      </RowCard>
    </Section>
  );
}
