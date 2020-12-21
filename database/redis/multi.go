package redis

// convertArgs transform []string keys to []interface{}, the latter is used as MULTI scope commands arguments
func convertArgs(args []string, foo func(string) string) []interface{} {
	qty := len(args)
	result := make([]interface{}, qty)

	if foo == nil {
		foo = identity
	}

	for i := 0; i < qty; i++ {
		result[i] = foo(args[i])
	}

	return result
}

func identity(val string) string {
	return val
}
