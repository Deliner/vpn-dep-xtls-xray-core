package packets

// This observer uses the imports of the exact reviewed server source. Error
// text and ICMP bodies are never logged; the original handler still owns both.
const serverErrorObserver = `
func nhopTraceServerWrite(snapshot string, queued, limit int, err error) {
	result := "write_error=" + nhopServerWriteOutcome(err) + " " + snapshot
	nhopTraceServerQueue("write_done", result, queued, limit, err != nil)
}

func nhopServerWriteOutcome(err error) string {
	_, tooBig := go_errors.AsType[*masque.PacketTooBigError](err)
	timeout, timed := go_errors.AsType[stdnet.Error](err)
	switch {
	case err == nil:
		return "none"
	case tooBig:
		return "packet-too-big"
	case go_errors.Is(err, stdnet.ErrClosed), go_errors.Is(err, io.ErrClosedPipe):
		return "closed"
	case go_errors.Is(err, context.Canceled):
		return "canceled"
	case go_errors.Is(err, context.DeadlineExceeded), timed && timeout.Timeout():
		return "timeout"
	default:
		return "other"
	}
}
`
