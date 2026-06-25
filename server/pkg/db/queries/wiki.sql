-- Wiki Space queries

-- name: CreateWikiSpace :one
INSERT INTO wiki_space (workspace_id, slug, display_name, access_scope, settings, template)
VALUES ($1, $2, $3, $4, $5, sqlc.narg('template'))
RETURNING *;

-- name: GetWikiSpace :one
SELECT * FROM wiki_space
WHERE workspace_id = $1 AND slug = $2 AND status = 'active';

-- name: ListWikiSpaces :many
SELECT * FROM wiki_space
WHERE workspace_id = $1 AND status = 'active'
ORDER BY CASE WHEN slug = 'default' THEN 0 ELSE 1 END, display_name, slug;

-- name: UpdateWikiSpace :one
UPDATE wiki_space SET
    display_name = COALESCE(sqlc.narg('display_name'), display_name),
    settings = CASE WHEN sqlc.narg('settings')::jsonb IS NOT NULL THEN settings || sqlc.narg('settings')::jsonb ELSE settings END,
    status = COALESCE(sqlc.narg('status'), status),
    default_agent_id = CASE WHEN sqlc.narg('default_agent_id')::uuid IS NOT NULL THEN sqlc.narg('default_agent_id')::uuid ELSE default_agent_id END,
    updated_at = now()
WHERE workspace_id = $1 AND slug = $2
RETURNING *;

-- name: ArchiveWikiSpace :exec
UPDATE wiki_space SET status = 'archived', updated_at = now()
WHERE workspace_id = $1 AND slug = $2 AND slug <> 'default';

-- name: EnsureWikiDefaultSpace :one
INSERT INTO wiki_space (workspace_id, slug, display_name, access_scope)
VALUES ($1, 'default', 'default', 'shared')
ON CONFLICT (workspace_id, slug)
DO UPDATE SET updated_at = wiki_space.updated_at
RETURNING *;

-- Wiki Page queries

-- name: CreateWikiPage :one
INSERT INTO wiki_page (space_id, path, title, page_type, content, frontmatter, backlinks, content_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpsertWikiPage :one
INSERT INTO wiki_page (space_id, path, title, page_type, content, frontmatter, backlinks, content_hash, current_revision_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, sqlc.narg('revision_id'))
ON CONFLICT (space_id, path)
DO UPDATE SET
    title = EXCLUDED.title,
    page_type = EXCLUDED.page_type,
    content = EXCLUDED.content,
    frontmatter = EXCLUDED.frontmatter,
    backlinks = EXCLUDED.backlinks,
    content_hash = EXCLUDED.content_hash,
    current_revision_id = COALESCE(EXCLUDED.current_revision_id, wiki_page.current_revision_id),
    updated_at = now()
RETURNING *;

-- name: GetWikiPageByPath :one
SELECT * FROM wiki_page
WHERE space_id = $1 AND path = $2;

-- name: GetWikiPageInSpace :one
SELECT wp.* FROM wiki_page wp
JOIN wiki_space ws ON ws.id = wp.space_id
WHERE ws.workspace_id = $1 AND wp.space_id = $2 AND wp.path = $3;

-- name: ListWikiPages :many
SELECT * FROM wiki_page
WHERE space_id = $1
ORDER BY path;

-- name: SearchWikiPages :many
SELECT path, title, page_type,
       ts_headline('english', content, plainto_tsquery('english', $2),
         'MaxWords=40, MinWords=20, ShortWord=3, MaxFragments=3, FragmentDelimiter=...') AS snippet
FROM wiki_page
WHERE space_id = $1
  AND to_tsvector('english', content) @@ plainto_tsquery('english', $2)
ORDER BY ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2)) DESC
LIMIT sqlc.narg('limit');

-- name: SetWikiPageValidationWarnings :exec
UPDATE wiki_page SET validation_warnings = $2::jsonb
WHERE space_id = $1 AND path = $3;

-- name: DeleteWikiPage :exec
DELETE FROM wiki_page
WHERE space_id = $1 AND path = $2;

-- name: ListWikiPagesByBacklink :many
SELECT path, title, page_type FROM wiki_page
WHERE space_id = $1 AND backlinks @> $2::jsonb;

-- Wiki Page Revision queries

-- name: CreateWikiPageRevision :one
INSERT INTO wiki_page_revision (page_id, space_id, operation_id, path, content, content_hash, summary)
VALUES ($1, $2, sqlc.narg('operation_id'), $3, $4, $5, sqlc.narg('summary'))
RETURNING *;

-- name: ListWikiPageRevisions :many
SELECT * FROM wiki_page_revision
WHERE space_id = $1 AND path = $2
ORDER BY created_at DESC
LIMIT sqlc.narg('limit');

-- Wiki Source queries

-- name: CreateWikiSource :one
INSERT INTO wiki_source (space_id, source_type, title, url, raw_path, content, content_hash, attachment_id, mime_type, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.narg('attachment_id'), sqlc.narg('mime_type'), $8)
RETURNING *;

-- name: GetWikiSource :one
SELECT * FROM wiki_source
WHERE id = $1 AND space_id = $2;

-- name: ListWikiSources :many
SELECT * FROM wiki_source
WHERE space_id = $1
ORDER BY created_at DESC;

-- name: UpdateWikiSourceStatus :exec
UPDATE wiki_source SET status = $2
WHERE id = $1 AND space_id = $3;

-- Wiki Operation queries

-- name: CreateWikiOperation :one
INSERT INTO wiki_operation (space_id, operation_type, status, hidden_issue_id, agent_session_id, metadata)
VALUES ($1, $2, 'pending', sqlc.narg('issue_id'), sqlc.narg('agent_session_id'), $3)
RETURNING *;

-- name: UpdateWikiOperationStatus :one
UPDATE wiki_operation SET
    status = $2,
    agent_session_id = COALESCE(sqlc.narg('agent_session_id'), agent_session_id),
    updated_at = now()
WHERE id = $1 AND space_id = $3
RETURNING *;

-- name: SetWikiOperationHiddenIssue :one
UPDATE wiki_operation SET
    hidden_issue_id = $2,
    updated_at = now()
WHERE id = $1 AND space_id = $3
RETURNING *;

-- name: CompleteWikiOperation :exec
UPDATE wiki_operation SET
    status = 'completed',
    cost_cents = COALESCE(sqlc.narg('cost_cents'), cost_cents),
    warnings = CASE WHEN sqlc.narg('warnings')::jsonb IS NOT NULL THEN warnings || sqlc.narg('warnings')::jsonb ELSE warnings END,
    affected_pages = CASE WHEN sqlc.narg('affected_pages')::jsonb IS NOT NULL THEN affected_pages || sqlc.narg('affected_pages')::jsonb ELSE affected_pages END,
    updated_at = now()
WHERE id = $1 AND space_id = $2;

-- name: GetWikiOperation :one
SELECT * FROM wiki_operation
WHERE id = $1;

-- name: ListWikiOperations :many
SELECT * FROM wiki_operation
WHERE space_id = $1
ORDER BY created_at DESC
LIMIT sqlc.narg('limit');

-- Wiki Query Session queries

-- name: CreateWikiQuerySession :one
INSERT INTO wiki_query_session (space_id, hidden_issue_id, agent_session_id)
VALUES ($1, sqlc.narg('issue_id'), sqlc.narg('agent_session_id'))
RETURNING *;

-- name: UpdateWikiQuerySession :exec
UPDATE wiki_query_session SET
    status = $2,
    agent_session_id = COALESCE(sqlc.narg('agent_session_id'), agent_session_id),
    filed_outputs = CASE WHEN sqlc.narg('filed_outputs')::jsonb IS NOT NULL THEN filed_outputs || sqlc.narg('filed_outputs')::jsonb ELSE filed_outputs END,
    updated_at = now()
WHERE id = $1;

-- name: GetWikiQuerySession :one
SELECT * FROM wiki_query_session
WHERE id = $1;
