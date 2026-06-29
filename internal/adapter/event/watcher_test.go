package event

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

func TestIsIncidentType_CrashLoopBackOff(t *testing.T) {
	typ, ok := IsIncidentType("CrashLoopBackOff")
	if !ok {
		t.Fatal("expected CrashLoopBackOff to be recognized")
	}
	if typ != incident.TypeCrashLoopBackOff {
		t.Errorf("expected TypeCrashLoopBackOff, got %s", typ)
	}
}

func TestIsIncidentType_OOMKilled(t *testing.T) {
	typ, ok := IsIncidentType("OOMKilled")
	if !ok {
		t.Fatal("expected OOMKilled to be recognized")
	}
	if typ != incident.TypeOOMKilled {
		t.Errorf("expected TypeOOMKilled, got %s", typ)
	}
}

func TestIsIncidentType_ImagePullBackOff(t *testing.T) {
	typ, ok := IsIncidentType("ImagePullBackOff")
	if !ok {
		t.Fatal("expected ImagePullBackOff to be recognized")
	}
	if typ != incident.TypeImagePullBackOff {
		t.Errorf("expected TypeImagePullBackOff, got %s", typ)
	}
}

func TestIsIncidentType_ErrImagePull(t *testing.T) {
	typ, ok := IsIncidentType("ErrImagePull")
	if !ok {
		t.Fatal("expected ErrImagePull to be recognized")
	}
	if typ != incident.TypeImagePullBackOff {
		t.Errorf("expected TypeImagePullBackOff, got %s", typ)
	}
}

func TestIsIncidentType_FailedScheduling(t *testing.T) {
	typ, ok := IsIncidentType("FailedScheduling")
	if !ok {
		t.Fatal("expected FailedScheduling to be recognized")
	}
	if typ != incident.TypeFailedScheduling {
		t.Errorf("expected TypeFailedScheduling, got %s", typ)
	}
}

func TestIsIncidentType_NodeNotReady(t *testing.T) {
	typ, ok := IsIncidentType("NodeNotReady")
	if !ok {
		t.Fatal("expected NodeNotReady to be recognized")
	}
	if typ != incident.TypeNodeNotReady {
		t.Errorf("expected TypeNodeNotReady, got %s", typ)
	}
}

func TestIsIncidentType_Unknown(t *testing.T) {
	_, ok := IsIncidentType("SomeUnknownReason")
	if ok {
		t.Fatal("expected unknown reason to not be recognized")
	}
}

func TestIsIncidentType_WithWhitespace(t *testing.T) {
	typ, ok := IsIncidentType("  CrashLoopBackOff  ")
	if !ok {
		t.Fatal("expected trimmed CrashLoopBackOff to be recognized")
	}
	if typ != incident.TypeCrashLoopBackOff {
		t.Errorf("expected TypeCrashLoopBackOff, got %s", typ)
	}
}

func TestDetectNodeIncident_NotReady(t *testing.T) {
	// detectNodeIncident requires a corev1.Node, tested via integration.
	// This test validates the IsIncidentType mapping used by the detection logic.
	typ, ok := IsIncidentType("NodeNotReady")
	if !ok || typ != incident.TypeNodeNotReady {
		t.Error("expected NodeNotReady to map correctly")
	}
}
