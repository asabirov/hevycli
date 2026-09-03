package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routineObjectBody is what POST /routines returns today: "routine" is a single
// object. Decoding it into []Routine is what produced
// "NETWORK_ERROR: failed to create routine: json: cannot unmarshal object into
// Go struct field RoutineResponse.routine of type []api.Routine" in production,
// after the routine had already been created.
const routineObjectBody = `{
  "routine": {
    "id": "d34e8f5c-1f2b-4a55-9a19-0a1f9c0b1234",
    "title": "Next Chest Workout",
    "folder_id": 2607603,
    "created_at": "2026-08-30T18:02:11Z",
    "updated_at": "2026-08-30T18:02:11Z",
    "exercises": [
      {
        "index": 0,
        "title": "Bench Press (Barbell)",
        "notes": "2 warm-up sets, then 3 work sets.",
        "exercise_template_id": "79D0BB3A",
        "sets": [
          {"index": 0, "type": "warmup", "weight_kg": 40, "reps": 15},
          {"index": 1, "type": "normal", "weight_kg": 80, "reps": 8}
        ]
      }
    ]
  }
}`

// routineArrayBody is the shape reported in obay/hevycli#2, which the client was
// changed to expect. Both shapes have been seen from the live API, so both decode.
const routineArrayBody = `{
  "routine": [
    {
      "id": "d34e8f5c-1f2b-4a55-9a19-0a1f9c0b1234",
      "title": "Next Chest Workout",
      "folder_id": 2607603,
      "created_at": "2026-08-30T18:02:11Z",
      "updated_at": "2026-08-30T18:02:11Z",
      "exercises": [
        {
          "index": 0,
          "title": "Bench Press (Barbell)",
          "exercise_template_id": "79D0BB3A",
          "sets": [{"index": 0, "type": "normal", "weight_kg": 80, "reps": 8}]
        }
      ]
    }
  ]
}`

func routineServer(t *testing.T, method, path, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, method, r.Method)
		assert.Equal(t, path, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestCreateRoutine_ObjectResponse(t *testing.T) {
	server := routineServer(t, http.MethodPost, "/routines", routineObjectBody, http.StatusCreated)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.CreateRoutine(&CreateRoutineRequest{
		Routine: CreateRoutineData{Title: "Next Chest Workout"},
	})

	require.NoError(t, err)
	require.NotNil(t, routine)
	assert.Equal(t, "d34e8f5c-1f2b-4a55-9a19-0a1f9c0b1234", routine.ID)
	assert.Equal(t, "Next Chest Workout", routine.Title)
	require.Len(t, routine.Exercises, 1)
	assert.Len(t, routine.Exercises[0].Sets, 2)
}

func TestCreateRoutine_ArrayResponse(t *testing.T) {
	server := routineServer(t, http.MethodPost, "/routines", routineArrayBody, http.StatusCreated)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.CreateRoutine(&CreateRoutineRequest{
		Routine: CreateRoutineData{Title: "Next Chest Workout"},
	})

	require.NoError(t, err)
	require.NotNil(t, routine)
	assert.Equal(t, "d34e8f5c-1f2b-4a55-9a19-0a1f9c0b1234", routine.ID)
	assert.Equal(t, "Next Chest Workout", routine.Title)
}

func TestUpdateRoutine_ObjectResponse(t *testing.T) {
	server := routineServer(t, http.MethodPut, "/routines/d34e8f5c", routineObjectBody, http.StatusOK)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.UpdateRoutine("d34e8f5c", &UpdateRoutineRequest{
		Routine: UpdateRoutineData{Title: "Next Chest Workout"},
	})

	require.NoError(t, err)
	require.NotNil(t, routine)
	assert.Equal(t, "Next Chest Workout", routine.Title)
}

func TestUpdateRoutine_ArrayResponse(t *testing.T) {
	server := routineServer(t, http.MethodPut, "/routines/d34e8f5c", routineArrayBody, http.StatusOK)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.UpdateRoutine("d34e8f5c", &UpdateRoutineRequest{
		Routine: UpdateRoutineData{Title: "Next Chest Workout"},
	})

	require.NoError(t, err)
	require.NotNil(t, routine)
	assert.Equal(t, "Next Chest Workout", routine.Title)
}

// A body the client cannot read is not a network failure. Labelling it
// NETWORK_ERROR is what made the original bug read as "the request did not
// happen", so an agent retried it seven times.
func TestCreateRoutine_UnreadableResponseIsNotNetworkError(t *testing.T) {
	server := routineServer(t, http.MethodPost, "/routines", `{"routine": 42}`, http.StatusCreated)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.CreateRoutine(&CreateRoutineRequest{
		Routine: CreateRoutineData{Title: "Next Chest Workout"},
	})

	require.Error(t, err)
	assert.Nil(t, routine)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, "INVALID_RESPONSE", apiErr.ErrorCode)
	assert.Contains(t, apiErr.Error(), "was created")
}

// routineBareBody is the shape Hevy's own OpenAPI document specifies for
// POST /v1/routines — 201 with schema $ref #/components/schemas/Routine and no
// "routine" envelope. The live API does wrap it, so the spec and the server
// disagree; the client reads both.
const routineBareBody = `{
  "id": "d34e8f5c-1f2b-4a55-9a19-0a1f9c0b1234",
  "title": "Next Chest Workout",
  "folder_id": null,
  "created_at": "2026-08-30T18:02:11Z",
  "updated_at": "2026-08-30T18:02:11Z",
  "exercises": []
}`

func TestCreateRoutine_BareObjectResponse(t *testing.T) {
	server := routineServer(t, http.MethodPost, "/routines", routineBareBody, http.StatusCreated)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.CreateRoutine(&CreateRoutineRequest{
		Routine: CreateRoutineData{Title: "Next Chest Workout"},
	})

	require.NoError(t, err)
	require.NotNil(t, routine)
	assert.Equal(t, "d34e8f5c-1f2b-4a55-9a19-0a1f9c0b1234", routine.ID)
}

func TestCreateRoutine_EmptyRoutineIsNotNetworkError(t *testing.T) {
	server := routineServer(t, http.MethodPost, "/routines", `{"routine": []}`, http.StatusCreated)
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	routine, err := client.CreateRoutine(&CreateRoutineRequest{
		Routine: CreateRoutineData{Title: "Next Chest Workout"},
	})

	require.Error(t, err)
	assert.Nil(t, routine)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, "INVALID_RESPONSE", apiErr.ErrorCode)
	assert.Contains(t, apiErr.Error(), "was created")
}
