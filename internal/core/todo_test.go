package core

import (
	"errors"
	"testing"
)

func TestCreatePlansEffects(t *testing.T) {
	effects, err := Create("t1", CreateInput{Title: "  buy milk  "})
	if err != nil || len(effects) != 2 {
		t.Fatalf("Create = %v, %v; want 2 effects", effects, err)
	}
	st, ok := effects[0].(StoreTodo)
	if !ok || st.Todo.ID != "t1" || st.Todo.Title != "buy milk" || st.Todo.Done {
		t.Fatalf("first effect = %#v; want trimmed StoreTodo{t1, buy milk, done=false}", effects[0])
	}
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	_, err := Create("t1", CreateInput{Title: "   "})
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("want ValidationError on title, got %v", err)
	}
}

func TestUpdateOnlyTouchesProvidedFields(t *testing.T) {
	cur := Todo{ID: "t1", Title: "old", Done: false}
	done := true
	effects, err := Update(cur, Patch{Done: &done}) // title omitted
	if err != nil {
		t.Fatal(err)
	}
	st := effects[0].(StoreTodo)
	if st.Todo.Title != "old" || !st.Todo.Done {
		t.Fatalf("got %#v; want title preserved, done=true", st.Todo)
	}
}

func TestUpdateRejectsEmptyTitle(t *testing.T) {
	empty := "  "
	if _, err := Update(Todo{ID: "t1"}, Patch{Title: &empty}); err == nil {
		t.Fatal("empty title in patch must be rejected")
	}
}

func TestDeletePlansEffect(t *testing.T) {
	effects, err := Delete("t1")
	if err != nil || len(effects) != 2 {
		t.Fatalf("Delete = %v, %v; want 2 effects", effects, err)
	}
	if _, ok := effects[0].(DeleteTodo); !ok {
		t.Fatalf("first effect = %#v; want DeleteTodo", effects[0])
	}
}
