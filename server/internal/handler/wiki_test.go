package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wiki-handler unit tests verify the no-config short-circuits: when
// WikiService is nil (wiki not configured), every endpoint returns
// 503 ServiceUnavailable.

func TestListWikiSpaces_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces", nil)
	w := httptest.NewRecorder()
	h.ListWikiSpaces(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCreateWikiSpace_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/x/wiki/spaces", nil)
	w := httptest.NewRecorder()
	h.CreateWikiSpace(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetWikiSpace_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default", nil)
	w := httptest.NewRecorder()
	h.GetWikiSpace(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestUpdateWikiSpace_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPatch, "/api/workspaces/x/wiki/spaces/default", nil)
	w := httptest.NewRecorder()
	h.UpdateWikiSpace(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestArchiveWikiSpace_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/x/wiki/spaces/default", nil)
	w := httptest.NewRecorder()
	h.ArchiveWikiSpace(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetWikiPage_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/pages/wiki/index.md", nil)
	w := httptest.NewRecorder()
	h.GetWikiPage(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListWikiPages_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/pages", nil)
	w := httptest.NewRecorder()
	h.ListWikiPages(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestUpsertWikiPage_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPut, "/api/workspaces/x/wiki/spaces/default/pages/wiki/test.md", nil)
	w := httptest.NewRecorder()
	h.UpsertWikiPage(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteWikiPage_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/x/wiki/spaces/default/pages/wiki/test.md", nil)
	w := httptest.NewRecorder()
	h.DeleteWikiPage(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestBatchReadWikiPages_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/x/wiki/spaces/default/pages/batch", nil)
	w := httptest.NewRecorder()
	h.BatchReadWikiPages(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestBatchWriteWikiPages_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/x/wiki/spaces/default/pages/batch-write", nil)
	w := httptest.NewRecorder()
	h.BatchWriteWikiPages(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListWikiPageRevisions_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/page-revisions/wiki%2Findex.md", nil)
	w := httptest.NewRecorder()
	h.ListWikiPageRevisions(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListWikiSources_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/sources", nil)
	w := httptest.NewRecorder()
	h.ListWikiSources(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCreateWikiSource_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/x/wiki/spaces/default/sources", nil)
	w := httptest.NewRecorder()
	h.CreateWikiSource(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetWikiSource_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/sources/x", nil)
	w := httptest.NewRecorder()
	h.GetWikiSource(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteWikiSource_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodDelete, "/api/workspaces/x/wiki/spaces/default/sources/x", nil)
	w := httptest.NewRecorder()
	h.DeleteWikiSource(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListWikiOperations_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/operations", nil)
	w := httptest.NewRecorder()
	h.ListWikiOperations(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCreateWikiOperation_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/x/wiki/spaces/default/operations", nil)
	w := httptest.NewRecorder()
	h.CreateWikiOperation(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetWikiOperation_NotConfigured(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/x/wiki/spaces/default/operations/x", nil)
	w := httptest.NewRecorder()
	h.GetWikiOperation(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
