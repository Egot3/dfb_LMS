package testrunner_test

import (
	"os"
	"testing"
	"time"

	"github.com/egot3/fathom/internal/carefulness"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
)

func TestRunner_Start(t *testing.T) {

	quizUUID, err := uuid.NewV7()
	require.NoError(t, err, "are we for real?")
	quizUUIDs := uuid.UUIDs{quizUUID}

	quizFile := testutils.TestQuiz(t)
	defer os.Remove(quizFile.Name())
	defer quizFile.Close()

	quizPathes := []string{quizFile.Name()}
	testUUID, err := uuid.NewV7()
	require.NoError(t, err)

	i := do.New()

	t.Run("Valid start", func(t *testing.T) {
		i := i.Scope("valid")
		do.Provide(i, testrunner.NewManager)

		m := do.MustInvoke[*testrunner.Manager](i)
		r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
		require.NoError(t, err)

		deadline := r.Deadline()
		require.NoError(t, err)
		require.WithinDuration(t, deadline, time.Now().Add(10*time.Second), 2*time.Second)
		require.Equal(t, r.GetAll(), quizUUIDs)
	})

	t.Run("Invalid start", func(t *testing.T) {
		t.Run("Orphans", func(t *testing.T) {
			testCases := []struct {
				desc       string
				quizPathes []string
				quizUUIDs  uuid.UUIDs
			}{
				{
					desc:       "No pathes",
					quizPathes: nil,
					quizUUIDs:  quizUUIDs,
				},
				{
					desc:       "No uuids",
					quizPathes: quizPathes,
					quizUUIDs:  nil,
				},
			}
			for _, tC := range testCases {
				t.Run(tC.desc, func(t *testing.T) {
					i := i.Scope(tC.desc)
					do.Provide(i, testrunner.NewManager)

					m := do.MustInvoke[*testrunner.Manager](i)
					_, err := m.Start(t.Context(), 10*time.Second, tC.quizPathes, tC.quizUUIDs, uuid.UUIDs{}, testUUID)
					require.Error(t, err)
					t.Log(err.Error())
				})
			}

		})

		t.Run("Not found", func(t *testing.T) {
			i := i.Scope("NotFound")
			do.Provide(i, testrunner.NewManager)

			m := do.MustInvoke[*testrunner.Manager](i)
			_, err := m.Start(t.Context(), 10*time.Second, []string{"/unknown.md"}, quizUUIDs, uuid.UUIDs{}, testUUID)
			require.Error(t, err)
		})

		t.Run("Not absolute", func(t *testing.T) {
			i := i.Scope("NotAbs")
			do.Provide(i, testrunner.NewManager)

			m := do.MustInvoke[*testrunner.Manager](i)
			_, err := m.Start(t.Context(), 10*time.Second, []string{"./unknown.md"}, quizUUIDs, uuid.UUIDs{}, testUUID)
			require.Error(t, err)
			require.ErrorIs(t, err, carefulness.ErrAbsoluteRequired)
		})
	})
}

func TestRunner_Get(t *testing.T) {

	quizUUID, err := uuid.NewV7()
	require.NoError(t, err, "are we for real?")
	quizUUIDs := uuid.UUIDs{quizUUID}

	quizFile := testutils.TestQuiz(t)
	defer os.Remove(quizFile.Name())
	defer quizFile.Close()

	quizPathes := []string{quizFile.Name()}
	testUUID, err := uuid.NewV7()
	require.NoError(t, err)

	i := do.New()
	do.Provide(i, testrunner.NewManager)

	t.Run("Valid get", func(t *testing.T) {
		i := i.Scope("valid")
		do.Provide(i, testrunner.NewManager)

		m := do.MustInvoke[*testrunner.Manager](i)
		r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
		require.NoError(t, err)

		q, err := r.Get(quizUUID)
		require.NoError(t, err)
		require.Equal(t, "there is a body!", q.Body)
		require.Equal(t, "quiz!", q.Title)
		require.Equal(t, "yeah!", q.Answer.Input.Input)
	})

	t.Run("Not found", func(t *testing.T) {
		i := i.Scope("NotFound")
		do.Provide(i, testrunner.NewManager)

		m := do.MustInvoke[*testrunner.Manager](i)
		r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
		require.NoError(t, err)

		q, err := r.Get(uuid.Nil)
		require.Error(t, err)
		require.Nil(t, q)

		require.ErrorIs(t, err, testrunner.ErrQuizNotCached)
	})
}

func TestRunner_Stop(t *testing.T) {

	quizUUID, err := uuid.NewV7()
	require.NoError(t, err, "are we for real?")
	quizUUIDs := uuid.UUIDs{quizUUID}

	quizFile := testutils.TestQuiz(t)
	defer os.Remove(quizFile.Name())
	defer quizFile.Close()

	quizPathes := []string{quizFile.Name()}
	testUUID, err := uuid.NewV7()
	require.NoError(t, err)

	i := do.New()
	do.Provide(i, testrunner.NewManager)

	t.Run("Valid stop", func(t *testing.T) {
		i := i.Scope("valid")
		do.Provide(i, testrunner.NewManager)

		m := do.MustInvoke[*testrunner.Manager](i)
		r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
		require.NoError(t, err)

		r.Stop()
		/* this doesn't even return an error
		because there is no "invalid stop"
		Server always have valid testrunner.TestRunner
		running multiple stops will do nothing */
		// you are not gonna believe this

		q, err := r.Get(quizUUID)
		require.Error(t, err)
		require.Nil(t, q)

		require.ErrorIs(t, err, testrunner.ErrRunnerInactive)
	})
}

// there were 2 test suite for methods which do not longer exist. They will be remembered

func TestRunner_Deadline(t *testing.T) {
	quizUUID := uuid.Must(uuid.NewV7())
	quizUUIDs := uuid.UUIDs{quizUUID}

	quizFile := testutils.TestQuiz(t)
	defer os.Remove(quizFile.Name())
	defer quizFile.Close()

	quizPathes := []string{quizFile.Name()}
	testUUID, err := uuid.NewV7()
	require.NoError(t, err)

	i := do.New()

	t.Run("Valid deadline", func(t *testing.T) {
		i := i.Scope("valid")
		do.Provide(i, testrunner.NewManager)

		m := do.MustInvoke[*testrunner.Manager](i)
		r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
		require.NoError(t, err)

		timeEnd := time.Now().Add(10 * time.Second)

		ti := r.Deadline()
		require.NoError(t, err)
		require.WithinDuration(t, timeEnd, ti, 2*time.Second)
	})
}

func TestRunner_Extend(t *testing.T) {
	quizUUID := uuid.Must(uuid.NewV7())
	quizUUIDs := uuid.UUIDs{quizUUID}

	quizFile := testutils.TestQuiz(t)
	defer os.Remove(quizFile.Name())
	defer quizFile.Close()

	quizPathes := []string{quizFile.Name()}
	testUUID, err := uuid.NewV7()
	require.NoError(t, err)

	i := do.New()

	t.Run("Valid extension", func(t *testing.T) {
		i := i.Scope("valid")
		do.Provide(i, testrunner.NewManager)

		m := do.MustInvoke[*testrunner.Manager](i)
		r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
		require.NoError(t, err)

		timeEnd := time.Now().Add(10 * time.Second)

		err = r.ExtendTime(time.Hour)
		require.NoError(t, err)

		ti := r.Deadline()
		require.NoError(t, err)
		require.WithinDuration(t, timeEnd.Add(time.Hour), ti, 10*time.Second)
	})
}

func TestRunner_PauseResume(t *testing.T) {

	i := do.New()

	t.Run("Valid pause resumance", func(t *testing.T) {

		t.Run("By waiting", func(t *testing.T) {

			i := i.Scope("wait")
			do.Provide(i, testrunner.NewManager)

			quizUUID := uuid.Must(uuid.NewV7())
			quizUUIDs := uuid.UUIDs{quizUUID}

			quizFile := testutils.TestQuiz(t)
			defer os.Remove(quizFile.Name())
			defer quizFile.Close()

			quizPathes := []string{quizFile.Name()}
			testUUID, err := uuid.NewV7()
			require.NoError(t, err)

			m := do.MustInvoke[*testrunner.Manager](i)
			r, err := m.Start(t.Context(), 10*time.Minute, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
			require.NoError(t, err)

			err = r.Pause()
			require.NoError(t, err)

			oldDeadline := r.Deadline()

			require.NoError(t, err)
			require.WithinDuration(t, time.Now().Add(10*time.Minute), oldDeadline, time.Second)

			t.Log("Please, stand by. Get yourself some tea")
			time.Sleep(10 * time.Second)
			t.Log("Thanks for your patience!")

			newNotch := r.Deadline()
			require.NoError(t, err)
			require.Equal(t, oldDeadline, newNotch)

			err = r.Resume()
			require.NoError(t, err)

			newNotch = r.Deadline()

			require.NoError(t, err)
			require.WithinDuration(t, oldDeadline.Add(10*time.Second), newNotch, 2*time.Second)
		})

		t.Run("By extending in the meantime", func(t *testing.T) {

			i := i.Scope("extending")
			do.Provide(i, testrunner.NewManager)

			quizUUID := uuid.Must(uuid.NewV7())
			quizUUIDs := uuid.UUIDs{quizUUID}

			quizFile := testutils.TestQuiz(t)
			defer os.Remove(quizFile.Name())
			defer quizFile.Close()

			quizPathes := []string{quizFile.Name()}
			testUUID, err := uuid.NewV7()
			require.NoError(t, err)

			m := do.MustInvoke[*testrunner.Manager](i)
			r, err := m.Start(t.Context(), 10*time.Minute, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
			require.NoError(t, err)

			err = r.Pause()
			require.NoError(t, err)

			oldDeadline := r.Deadline()

			require.WithinDuration(t, time.Now().Add(10*time.Minute), oldDeadline, time.Second)

			err = r.ExtendTime(10 * time.Second)

			newNotch := r.Deadline()
			require.NoError(t, err)
			require.NotEqual(t, oldDeadline, newNotch)
			require.Equal(t, oldDeadline.Add(10*time.Second), newNotch)

			err = r.Resume()
			require.NoError(t, err)

			newNotch = r.Deadline()

			require.WithinDuration(t, oldDeadline.Add(10*time.Second), newNotch, 2*time.Second)
		})
	})

	t.Run("Invalid pause+resume sequence", func(t *testing.T) {

		t.Run("Pausing", func(t *testing.T) {

			t.Run("Paused", func(t *testing.T) {
				i := i.Scope("paused")
				do.Provide(i, testrunner.NewManager)

				quizUUID := uuid.Must(uuid.NewV7())
				quizUUIDs := uuid.UUIDs{quizUUID}

				quizFile := testutils.TestQuiz(t)
				defer os.Remove(quizFile.Name())
				defer quizFile.Close()

				quizPathes := []string{quizFile.Name()}
				testUUID, err := uuid.NewV7()
				require.NoError(t, err)

				m := do.MustInvoke[*testrunner.Manager](i)
				r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
				require.NoError(t, err)

				err = r.Pause()
				require.NoError(t, err)

				err = r.Pause()
				require.Error(t, err)
				require.ErrorIs(t, err, testrunner.ErrRunnerPaused)
			})
		})

		t.Run("Resuming", func(t *testing.T) {

			i := do.New()

			t.Run("Running", func(t *testing.T) {
				i := i.Scope("running")
				do.Provide(i, testrunner.NewManager)

				quizUUID := uuid.Must(uuid.NewV7())
				quizUUIDs := uuid.UUIDs{quizUUID}

				quizFile := testutils.TestQuiz(t)
				defer os.Remove(quizFile.Name())
				defer quizFile.Close()

				quizPathes := []string{quizFile.Name()}
				testUUID, err := uuid.NewV7()
				require.NoError(t, err)

				m := do.MustInvoke[*testrunner.Manager](i)
				r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
				require.NoError(t, err)

				err = r.Pause()
				require.NoError(t, err)

				err = r.Resume()
				require.NoError(t, err)

				err = r.Resume()
				require.Error(t, err)
				require.ErrorIs(t, err, testrunner.ErrRunnerNotPaused)
			})
		})

		t.Run("Expired", func(t *testing.T) {

			t.Run("Expiration during pause", func(t *testing.T) {
				i := i.Scope("ExpDuringPause")
				do.Provide(i, testrunner.NewManager)

				quizUUID := uuid.Must(uuid.NewV7())
				quizUUIDs := uuid.UUIDs{quizUUID}

				quizFile := testutils.TestQuiz(t)
				defer os.Remove(quizFile.Name())
				defer quizFile.Close()

				quizPathes := []string{quizFile.Name()}
				testUUID, err := uuid.NewV7()
				require.NoError(t, err)

				m := do.MustInvoke[*testrunner.Manager](i)
				r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
				require.NoError(t, err)

				err = r.Pause()
				require.NoError(t, err)

				deadline := r.Deadline()

				t.Log("Please, stand by. Get yourself some tea")
				time.Sleep(5 * time.Second)
				t.Log("Thanks for your patience!")

				err = r.Resume()
				require.NoError(t, err)

				newDeadline := r.Deadline()

				require.WithinDuration(t, deadline, newDeadline, 6*time.Second)
			})
			t.Run("Manual expiration during pause", func(t *testing.T) {
				i := do.New()
				do.Provide(i, testrunner.NewManager)

				quizUUID := uuid.Must(uuid.NewV7())
				quizUUIDs := uuid.UUIDs{quizUUID}

				quizFile := testutils.TestQuiz(t)
				defer os.Remove(quizFile.Name())
				defer quizFile.Close()

				quizPathes := []string{quizFile.Name()}
				testUUID, err := uuid.NewV7()
				require.NoError(t, err)

				m := do.MustInvoke[*testrunner.Manager](i)
				r, err := m.Start(t.Context(), 10*time.Second, quizPathes, quizUUIDs, uuid.UUIDs{}, testUUID)
				require.NoError(t, err)

				err = r.Pause()
				require.NoError(t, err)

				err = r.ExtendTime(-20 * time.Minute) //yes, VSCode, -20*time.Saturday
				require.NoError(t, err)

				err = r.Resume()
				require.Error(t, err)
				require.ErrorIs(t, err, testrunner.ErrRunnerExpired)
			})
		})
	})
}
