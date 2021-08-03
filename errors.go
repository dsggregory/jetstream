package jetstream

type ErrTimeout struct{}

func (e ErrTimeout) Error() string {
	return "timed-out"
}

type ErrNotImplemented struct{}

func (e ErrNotImplemented) Error() string {
	return "not implemented"
}

type ErrSubjectConfigMismatch struct {
	str string
}

func (e ErrSubjectConfigMismatch) Error() string {
	return e.str
}

type ErrBadConfig struct {
	str string
}

func (e ErrBadConfig) Error() string {
	return e.str
}

type ErrUnsupportedType struct {
	str string
}

func (e ErrUnsupportedType) Error() string {
	return e.str
}
