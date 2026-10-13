package jobs

import "context"

// Book records a job before and after the work.
type Book interface {
	StartJob(kind string) (string, error)
	FinishJob(id, status, message string) error
}

// Run marks the job running, then succeeded or failed.
// The job row remains after the function returns.
func Run(ctx context.Context, book Book, kind string, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := book.StartJob(kind)
	if err != nil {
		return err
	}
	err = fn(ctx)
	status, message := "succeeded", ""
	if err != nil {
		status, message = "failed", err.Error()
	}
	if fin := book.FinishJob(id, status, message); fin != nil {
		if err != nil {
			return err
		}
		return fin
	}
	return err
}
