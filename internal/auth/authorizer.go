package auth

import (
	"fmt"

	"github.com/casbin/casbin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Authorizer is a wrapper for a casbin enforcer.
type Authorizer struct {
	enforcer *casbin.Enforcer
}

// New instantiates a new authorizer using the model
// and policy specified in the files stored in the
// given model and policy file paths.
func New(model, policy string) *Authorizer {
	enforcer := casbin.NewEnforcer(model, policy)
	return &Authorizer{
		enforcer: enforcer,
	}
}

// Authorize verifies if a given subject is
// permitted to run a given action on a given
// object.
func (a *Authorizer) Authorize(
	subject string,
	object string,
	action string,
) error {
	if !a.enforcer.Enforce(
		subject,
		object,
		action) {
		msg := fmt.Sprintf(
			"%s not permitted to %s to %s",
			subject,
			action,
			object,
		)

		st := status.New(codes.PermissionDenied, msg)
		return st.Err()
	}

	return nil
}
