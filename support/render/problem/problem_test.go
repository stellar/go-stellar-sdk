package problem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	ge "github.com/go-errors/errors"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/test"
	"github.com/stretchr/testify/assert"
)

// TestProblemRender tests the render cases
func TestProblemRender(t *testing.T) {
	problem := New("", log.DefaultLogger, LogNoErrors)

	testCases := []struct {
		name     string
		p        P
		wantList []string
		wantCode int
	}{
		{
			"server error",
			ServerError,
			[]string{"500"},
			500,
		}, {
			"renders the type correctly",
			P{Type: "foo", Status: 500},
			[]string{"foo"},
			500,
		}, {
			"renders the status correctly",
			P{Status: 201},
			[]string{"201"},
			201,
		}, {
			"renders the extras correctly",
			P{Extras: map[string]interface{}{"hello": "stellar"}, Status: 500},
			[]string{"hello", "stellar"},
			500,
		},
	}

	for _, kase := range testCases {
		t.Run(kase.name, func(t *testing.T) {
			w := testProblemRender(context.Background(), problem, kase.p)
			for _, wantItem := range kase.wantList {
				assert.True(t, strings.Contains(w.Body.String(), wantItem), w.Body.String())
				assert.Equal(t, kase.wantCode, w.Code)
			}
		})
	}
}

// TestProblemServerErrorConversion tests that we convert errors to ServerError problems and also log the
// stacktrace as unknown for non-rich errors
func TestProblemServerErrorConversion(t *testing.T) {
	problem := New("", log.DefaultLogger, LogUnknownErrors)

	testCases := []struct {
		name          string
		err           error
		wantSubstring string
	}{
		{
			"non-rich errors",
			errors.New("broke"),
			"stack=unknown", // logs the stacktrace as unknown for non-rich errors
		}, {
			"rich errors",
			ge.New("broke"),
			"problem_test.go:",
		},
	}

	for _, kase := range testCases {
		t.Run(kase.name, func(t *testing.T) {
			ctx, buf := test.ContextWithLogBuffer()
			w := testProblemRender(ctx, problem, kase.err)
			logged := buf.String()

			assert.True(t, strings.Contains(w.Body.String(), "server_error"), w.Body.String())
			assert.Equal(t, 500, w.Code)

			// don't expose private error info in the response body
			assert.False(t, strings.Contains(w.Body.String(), "broke"), w.Body.String())

			// log additional information about the error
			assert.True(
				t,
				strings.Contains(logged, kase.wantSubstring),
				fmt.Sprintf("expecting substring: '%s' in '%s'", kase.wantSubstring, logged),
			)
		})
	}
}

// TestProblemInflate test errors that come inflated from horizon
func TestProblemInflate(t *testing.T) {
	problem := New("", log.DefaultLogger, LogNoErrors)

	testCase := struct {
		name string
		p    P
		want string
	}{
		"renders the type correctly",
		P{Type: "https://stellar.org/horizon-errors/not_found", Status: 404},
		"https://stellar.org/horizon-errors/not_found",
	}

	t.Run(testCase.name, func(t *testing.T) {
		w := testProblemRender(context.Background(), problem, testCase.p)
		var payload P
		err := json.Unmarshal([]byte(w.Body.String()), &payload)
		if assert.NoError(t, err) {
			assert.Equal(t, testCase.want, payload.Type)
		}
	})
}

func testProblemRender(ctx context.Context, problem *Problem, err error) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	problem.Render(ctx, w, err)
	return w
}

func TestProblemRegisterReportFunc(t *testing.T) {
	problem := New("", log.DefaultLogger, LogAllErrors)

	var buf strings.Builder
	ctx := context.Background()

	reportFunc := func(ctx context.Context, err error) {
		buf.WriteString("captured ")
		buf.WriteString(err.Error())
	}

	err := errors.New("an unexpected error")

	w := httptest.NewRecorder()

	// before register the reportFunc
	problem.Render(ctx, w, err)
	assert.Equal(t, "", buf.String())

	problem.RegisterReportFunc(reportFunc)
	defer problem.RegisterReportFunc(nil)

	// after register the reportFunc
	want := "captured an unexpected error"
	problem.Render(ctx, w, err)
	assert.Equal(t, want, buf.String())
}

func TestProblemUnRegisterErrors(t *testing.T) {
	problem := New("", log.DefaultLogger, LogNoErrors)

	problem.RegisterError(context.DeadlineExceeded, ServerError)
	err := problem.IsKnownError(context.DeadlineExceeded)
	assert.Error(t, err, ServerError.Error())

	problem.UnRegisterErrors()

	err = problem.IsKnownError(context.DeadlineExceeded)
	assert.NoError(t, err)
}

func TestProblemIsKnownError(t *testing.T) {
	problem := New("", log.DefaultLogger, LogNoErrors)

	problem.RegisterError(context.DeadlineExceeded, ServerError)
	defer problem.UnRegisterErrors()
	err := problem.IsKnownError(context.DeadlineExceeded)
	assert.Error(t, err, ServerError.Error())

	err = problem.IsKnownError(errors.New("foo"))
	assert.NoError(t, err)
}

func TestErrorIncludesPInformation(t *testing.T) {
	err_str := ServerError.Error()
	assert.True(t, strings.Contains(err_str, ServerError.Detail))
	assert.True(t, strings.Contains(err_str, ServerError.Type))
}
