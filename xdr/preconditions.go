package xdr

// NewPreconditionsWithTimeBounds constructs the simplest possible
// `Preconditions` instance given the (possibly empty) timebounds.
func NewPreconditionsWithTimeBounds(timebounds *TimeBounds) Preconditions {
	cond := Preconditions{Type: PreconditionTypePrecondNone}
	if timebounds != nil {
		cond.Type = PreconditionTypePrecondTime
		cond.TimeBounds = timebounds
	}
	return cond
}
