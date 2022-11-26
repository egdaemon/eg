package cmderrors

import (
	"errors"
	"fmt"
	"log"

	"github.com/logrusorgru/aurora"
)

func Sprint(err error) string {
	type NotificationError interface {
		Notification()
	}

	type ShortError interface {
		UserFriendly()
	}

	var (
		nErr NotificationError
		sErr ShortError
	)

	if errors.As(err, &nErr) {
		return fmt.Sprint(err)
	}

	if errors.As(err, &sErr) {
		return fmt.Sprint(aurora.NewAurora(true).Red("ERROR"), err)
	}

	return fmt.Sprintf("%T - [%+v]", err, err)
}

// LogCause returns a string format based on the verbosity.
func LogCause(err error) error {
	type NotificationError interface {
		Notification()
	}

	type ShortError interface {
		UserFriendly()
	}

	var (
		nErr NotificationError
		sErr ShortError
	)

	if err == nil {
		return nil
	}

	if errors.As(err, &nErr) {
		log.Println(err)
	} else if errors.As(err, &sErr) {
		log.Println(aurora.NewAurora(true).Red("ERROR"), err)
	} else {
		log.Printf("%T - [%+v]\n", err, err)
	}

	return err
}
