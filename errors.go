package discov

var errChan = make(chan error, 1<<6)

func CatchRuntimeErrors() <-chan error {
	return errChan
}

func pushError(err error) {
	select {
	case errChan <- err:
	default: // errChan is full, discard err
	}
}
