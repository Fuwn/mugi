package remote

type Operation int

const (
	Pull Operation = iota
	Push
	Fetch
)

func (o Operation) String() string {
	switch o {
	case Pull:
		return "pull"
	case Push:
		return "push"
	case Fetch:
		return "fetch"
	default:
		return "unknown"
	}
}

func (o Operation) Verb() string {
	switch o {
	case Pull:
		return "Pulling"
	case Push:
		return "Pushing"
	case Fetch:
		return "Fetching"
	default:
		return "Operating"
	}
}

func (o Operation) PastTense() string {
	switch o {
	case Pull:
		return "Pulled"
	case Push:
		return "Pushed"
	case Fetch:
		return "Fetched"
	default:
		return "Completed"
	}
}

const All = "all"
