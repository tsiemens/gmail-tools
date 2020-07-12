package api

type Label struct {
	name string
	id   string
}

func NewLabelWithName(name string) Label {
	return Label{name: name}
}

func NewLabelWithId(id string) Label {
	return Label{id: id}
}

func LabelsFromLabelNames(labelNames []string) []Label {
	labels := make([]Label, 0, len(labelNames))
	for _, ln := range labelNames {
		labels = append(labels, NewLabelWithName(ln))
	}
	return labels
}

func (l Label) String() string {
	if l.name != "" {
		return l.name
	}
	return l.id
}

// Constant Labels
var InboxLabel = NewLabelWithId("INBOX")
var TrashLabel = NewLabelWithId("TRASH")
