package types

import (
	"errors"
	"fmt"
)

var (
	ErrFinalityProviderAlreadyExisted = errors.New("the finality provider has already existed")
	ErrEOTSManagerServerNoRespond     = fmt.Errorf("the EOTS manager server is not responding")
)
