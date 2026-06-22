-- Add wiki core tables: wiki_space, wiki_page, wiki_page_revision,
-- wiki_source, wiki_operation, wiki_query_session.

CREATE TABLE wiki_space (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    slug TEXT NOT NULL DEFAULT 'default',
    display_name TEXT NOT NULL,
    access_scope TEXT NOT NULL DEFAULT 'shared' CHECK (access_scope IN ('shared', 'personal')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    settings JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, slug)
);

CREATE INDEX idx_wiki_space_workspace ON wiki_space(workspace_id, status);

CREATE TABLE wiki_page (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    title TEXT,
    page_type TEXT,
    content TEXT NOT NULL DEFAULT '',
    frontmatter JSONB NOT NULL DEFAULT '{}',
    backlinks JSONB NOT NULL DEFAULT '[]',
    content_hash TEXT NOT NULL,
    current_revision_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (space_id, path)
);

CREATE INDEX idx_wiki_page_space_path ON wiki_page(space_id, path);
CREATE INDEX idx_wiki_page_space_type ON wiki_page(space_id, page_type);
CREATE INDEX idx_wiki_page_fts ON wiki_page USING gin (to_tsvector('english', content));
CREATE INDEX idx_wiki_page_backlinks ON wiki_page USING gin (backlinks);

CREATE TABLE wiki_page_revision (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES wiki_page(id) ON DELETE CASCADE,
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    operation_id UUID,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_page_revision_page ON wiki_page_revision(page_id, created_at DESC);

CREATE TABLE wiki_source (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL DEFAULT 'text',
    title TEXT NOT NULL,
    url TEXT,
    raw_path TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL,
    attachment_id UUID REFERENCES attachment(id) ON DELETE SET NULL,
    mime_type TEXT,
    status TEXT NOT NULL DEFAULT 'captured' CHECK (status IN ('captured', 'ingested', 'archived')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_source_space_status ON wiki_source(space_id, status);
CREATE INDEX idx_wiki_source_space_path ON wiki_source(space_id, raw_path);

CREATE TABLE wiki_operation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('ingest', 'query', 'lint', 'distill', 'index')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    hidden_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    agent_session_id TEXT,
    run_ids JSONB NOT NULL DEFAULT '[]',
    cost_cents INTEGER NOT NULL DEFAULT 0,
    warnings JSONB NOT NULL DEFAULT '[]',
    affected_pages JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_operation_space_type_status ON wiki_operation(space_id, operation_type, status);

CREATE TABLE wiki_query_session (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    hidden_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    agent_session_id TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'filed')),
    filed_outputs JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_query_session_space ON wiki_query_session(space_id, updated_at DESC);
