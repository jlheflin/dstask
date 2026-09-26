package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvedTasksGetOwnIDs(t *testing.T) {
	repo, cleanup := makeDstaskRepo(t)
	defer cleanup()

	program := testCmd(repo)

	for _, summary := range []string{"first", "second", "third"} {
		output, exiterr, success := program("add", summary)
		assertProgramResult(t, output, exiterr, success)
	}

	// resolve in a known order: r1 = second, r2 = first
	output, exiterr, success := program("2", "done")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("1", "done")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("show-resolved")
	assertProgramResult(t, output, exiterr, success)

	tasks := unmarshalTaskArray(t, output)
	require.Len(t, tasks, 2)
	assert.Equal(t, "second", tasks[0].Summary)
	assert.Equal(t, 1, tasks[0].ResolvedID)
	assert.Equal(t, "first", tasks[1].Summary)
	assert.Equal(t, 2, tasks[1].ResolvedID)

	// a new open task reuses ID 1, but r1 still addresses the resolved task
	output, exiterr, success = program("add", "fourth")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("r1")
	assertProgramResult(t, output, exiterr, success)

	tasks = unmarshalTaskArray(t, output)
	require.Len(t, tasks, 1)
	assert.Equal(t, "second", tasks[0].Summary)
}

func TestModifyAndNoteResolvedTask(t *testing.T) {
	repo, cleanup := makeDstaskRepo(t)
	defer cleanup()

	program := testCmd(repo)

	output, exiterr, success := program("add", "write report")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("1", "done")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("r1", "modify", "+archive", "project:work")
	assertProgramResult(t, output, exiterr, success)

	_, exiterr, success = program("r1", "note", "sent to Alex on Friday")
	assertProgramResult(t, nil, exiterr, success)

	output, exiterr, success = program("show-resolved")
	assertProgramResult(t, output, exiterr, success)

	tasks := unmarshalTaskArray(t, output)
	require.Len(t, tasks, 1)
	assert.Equal(t, "resolved", tasks[0].Status)
	assert.Equal(t, []string{"archive"}, tasks[0].Tags)
	assert.Equal(t, "work", tasks[0].Project)
	assert.Equal(t, "sent to Alex on Friday", tasks[0].Notes)
}

func TestReopenResolvedTask(t *testing.T) {
	repo, cleanup := makeDstaskRepo(t)
	defer cleanup()

	program := testCmd(repo)

	output, exiterr, success := program("add", "fix the gutter")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("1", "done")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("add", "buy nails")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("r1", "reopen")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("show-open")
	assertProgramResult(t, output, exiterr, success)

	tasks := unmarshalTaskArray(t, output)
	require.Len(t, tasks, 2)

	byID := map[int]string{}
	for _, task := range tasks {
		byID[task.ID] = task.Summary
		assert.Equal(t, "pending", task.Status)
		assert.True(t, task.Resolved.IsZero())
	}

	assert.Equal(t, "buy nails", byID[1])
	assert.Equal(t, "fix the gutter", byID[2])

	output, exiterr, success = program("show-resolved")
	assertProgramResult(t, output, exiterr, success)
	assert.Empty(t, unmarshalTaskArray(t, output))

	// reopening an open task is an error
	_, _, success = program("r1", "reopen")
	assert.False(t, success)
}

func TestResolvedIDsArePermanent(t *testing.T) {
	repo, cleanup := makeDstaskRepo(t)
	defer cleanup()

	program := testCmd(repo)

	for _, summary := range []string{"a", "b", "c", "d"} {
		output, exiterr, success := program("add", summary)
		assertProgramResult(t, output, exiterr, success)
	}

	// resolve a, b, c in order: r1, r2, r3
	for _, id := range []string{"1", "2", "3"} {
		output, exiterr, success := program(id, "done")
		assertProgramResult(t, output, exiterr, success)
	}

	output, exiterr, success := program("show-resolved")
	assertProgramResult(t, output, exiterr, success)
	require.Len(t, unmarshalTaskArray(t, output), 3)

	// reopening r1 must not renumber b and c
	output, exiterr, success = program("r1", "reopen")
	assertProgramResult(t, output, exiterr, success)

	// resolving d, then a again, gets new numbers; r1 is retired
	output, exiterr, success = program("4", "done")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("show-open")
	assertProgramResult(t, output, exiterr, success)
	open := unmarshalTaskArray(t, output)
	require.Len(t, open, 1)
	assert.Equal(t, "a", open[0].Summary)

	output, exiterr, success = program(open[0].Ref(), "done")
	assertProgramResult(t, output, exiterr, success)

	output, exiterr, success = program("show-resolved")
	assertProgramResult(t, output, exiterr, success)

	got := map[string]int{}
	for _, task := range unmarshalTaskArray(t, output) {
		got[task.Summary] = task.ResolvedID
	}

	assert.Equal(t, map[string]int{"b": 2, "c": 3, "d": 4, "a": 5}, got)

	// retired number r1 no longer addresses any task
	output, exiterr, success = program("r1")
	assertProgramResult(t, output, exiterr, success)
	assert.Empty(t, unmarshalTaskArray(t, output))

	_, _, success = program("r1", "note", "should fail")
	assert.False(t, success)
}
