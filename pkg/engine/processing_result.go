package engine

type ProcessingOutcome string

const (
	ProcessingOutcomeOK ProcessingOutcome = "ok"
)

type ProcessingException string

const (
	ProcessingExceptionNone         ProcessingException = ""
	ProcessingExceptionInvalidInput ProcessingException = "invalid_input"
	ProcessingExceptionMissingInput ProcessingException = "missing_input"
	ProcessingExceptionParse        ProcessingException = "parse_failed"
	ProcessingExceptionFinalize     ProcessingException = "finalize_failed"
	ProcessingExceptionRetention    ProcessingException = "retention_failed"
	ProcessingExceptionCancelled    ProcessingException = "cancelled"
)

type FeedProcessingResult struct {
	Outcome   ProcessingOutcome
	Exception ProcessingException
	Processed bool
	Message   string
	Err       error
	Work      FeedProcessingWork
}

type FeedProcessingWork struct {
	InputBytes int64
	Entries    int64
	UniqueIPs  int64
}

func processingOK(message string, processed bool) FeedProcessingResult {
	return FeedProcessingResult{
		Outcome:   ProcessingOutcomeOK,
		Exception: ProcessingExceptionNone,
		Processed: processed,
		Message:   message,
	}
}

func (r FeedProcessingResult) withWork(work FeedProcessingWork) FeedProcessingResult {
	r.Work = work
	return r
}

func processingException(exception ProcessingException, message string, err error) FeedProcessingResult {
	return FeedProcessingResult{
		Outcome:   ProcessingOutcomeOK,
		Exception: exception,
		Message:   message,
		Err:       err,
	}
}

func (r FeedProcessingResult) StatusString() string {
	if r.Exception != ProcessingExceptionNone {
		return r.Exception.String()
	}
	return r.Outcome.String()
}

func (e ProcessingException) String() string {
	return string(e)
}

func (o ProcessingOutcome) String() string {
	return string(o)
}
