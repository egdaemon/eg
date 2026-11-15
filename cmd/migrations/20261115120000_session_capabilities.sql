-- +goose Up
-- +goose StatementBegin
ALTER TABLE serve_sessions ADD COLUMN IF NOT EXISTS cap_ssh BOOLEAN DEFAULT false; -- reachable over ssh.
ALTER TABLE serve_sessions ADD COLUMN IF NOT EXISTS cap_agent BOOLEAN DEFAULT false; -- an agent can run inside it.
ALTER TABLE serve_sessions ADD COLUMN IF NOT EXISTS cap_patch BOOLEAN DEFAULT false; -- its changes are patched back to the worktree.
-- sessions recorded before the capabilities existed were all served with ssh, which auto patches.
UPDATE serve_sessions SET cap_ssh = true, cap_agent = true, cap_patch = true WHERE ssh_requested >= 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE serve_sessions DROP COLUMN IF EXISTS cap_patch;
ALTER TABLE serve_sessions DROP COLUMN IF EXISTS cap_agent;
ALTER TABLE serve_sessions DROP COLUMN IF EXISTS cap_ssh;
-- +goose StatementEnd
