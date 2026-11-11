// Keeps one page's error from blanking the whole window: the page shows
// what went wrong and a way to draw it again; the sidebar and the Agent
// panel stay.

import { Component, type ReactNode } from 'react';
import { Alert, Box, Button } from '@mui/material';

export class PageBoundary extends Component<{ children: ReactNode; label: string; retry: string; resetKey: string }, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidUpdate(prev: { resetKey: string }) {
    if (prev.resetKey !== this.props.resetKey && this.state.error) this.setState({ error: null });
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <Box sx={{ p: 3 }}>
        <Alert
          severity="error"
          action={
            <Button color="inherit" onClick={() => this.setState({ error: null })}>
              {this.props.retry}
            </Button>
          }
        >
          {this.props.label}
          <Box component="pre" sx={{ m: 0, mt: 1, fontSize: 12, whiteSpace: 'pre-wrap', userSelect: 'text' }}>
            {this.state.error.message}
          </Box>
        </Alert>
      </Box>
    );
  }
}
