package record

import (
	"reflect"
	"testing"
)

func TestArgs(t *testing.T) {
	got := (Recorder{Target: "source-name"}).Args("out.wav")
	want := []string{"--rate=16000", "--channels=1", "--format=s16", "--channel-map=MONO", "--target=source-name", "out.wav"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
