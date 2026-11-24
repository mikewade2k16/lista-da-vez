package tasks

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const testCommentTaskID = "cccccccc-cccc-cccc-cccc-cccccccccccc"

func assertNoActiveHTML(t *testing.T, label, html string) {
	t.Helper()
	lower := strings.ToLower(html)
	for _, bad := range []string{"<script", "onerror", "javascript:", "<iframe", "onclick"} {
		if strings.Contains(lower, bad) {
			t.Fatalf("%s: %q sobreviveu em %q", label, bad, html)
		}
	}
}

func TestAddCommentSanitizesBodyHTMLBeforeStoring(t *testing.T) {
	payloads := []string{
		`<p>oi</p><script>alert(1)</script>`,
		`<img src=x onerror="alert(1)">`,
		`<a href="javascript:alert(1)">clique</a>`,
	}
	for _, payload := range payloads {
		stored := ""
		repository := &repositoryMock{
			onGetTask: func(_ context.Context, _ AccessContext, taskID string) (Task, error) {
				return Task{ID: taskID}, nil
			},
			onAddComment: func(_ context.Context, _ string, input AddCommentInput, _ string) (Comment, error) {
				stored = input.BodyHTML
				return Comment{ID: "c1", TaskID: input.TaskID, BodyHTML: input.BodyHTML}, nil
			},
		}
		service := NewService(repository, nil, nil, nil)
		comment, err := service.AddComment(context.Background(), agencyAccess(), AddCommentInput{TaskID: testCommentTaskID, BodyHTML: payload})
		if err != nil && !errors.Is(err, ErrValidation) {
			t.Fatalf("AddComment(%q): %v", payload, err)
		}
		assertNoActiveHTML(t, "gravado", stored)
		assertNoActiveHTML(t, "resposta", comment.BodyHTML)
	}
}

func TestAddCommentKeepsAllowedFormatting(t *testing.T) {
	stored := ""
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, taskID string) (Task, error) {
			return Task{ID: taskID}, nil
		},
		onAddComment: func(_ context.Context, _ string, input AddCommentInput, _ string) (Comment, error) {
			stored = input.BodyHTML
			return Comment{ID: "c1", BodyHTML: input.BodyHTML}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)
	html := `<p><strong>ok</strong> <a href="https://omni.example/x">link</a> <span class="mention" data-id="u1">@ana</span></p><ul><li>a</li></ul>`
	if _, err := service.AddComment(context.Background(), agencyAccess(), AddCommentInput{TaskID: testCommentTaskID, BodyHTML: html}); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	for _, keep := range []string{"<strong>ok</strong>", `href="https://omni.example/x"`, `data-id="u1"`, "<li>a</li>"} {
		if !strings.Contains(stored, keep) {
			t.Fatalf("perdeu %q em %q", keep, stored)
		}
	}
}

func TestAddCommentEmptyAfterSanitizeIsValidationError(t *testing.T) {
	repository := &repositoryMock{}
	service := NewService(repository, nil, nil, nil)
	_, err := service.AddComment(context.Background(), agencyAccess(), AddCommentInput{TaskID: testCommentTaskID, BodyHTML: `<script>alert(1)</script>`})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("esperava ErrValidation, veio %v", err)
	}
}

func TestListCommentsSanitizesLegacyRows(t *testing.T) {
	repository := &repositoryMock{
		onListComments: func(_ context.Context, _ AccessContext, _ string) ([]Comment, error) {
			return []Comment{
				{ID: "1", BodyHTML: `<img src=x onerror="alert(1)"><p>a</p>`},
				{ID: "2", BodyHTML: `<a href="javascript:alert(1)">x</a><script>1</script>`},
				{ID: "3", BodyHTML: `<p><em>ok</em></p>`},
			}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)
	comments, err := service.ListComments(context.Background(), agencyAccess(), testCommentTaskID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	for _, comment := range comments {
		assertNoActiveHTML(t, comment.ID, comment.BodyHTML)
	}
	if !strings.Contains(comments[2].BodyHTML, "<em>ok</em>") {
		t.Fatalf("formatacao perdida: %q", comments[2].BodyHTML)
	}
}
