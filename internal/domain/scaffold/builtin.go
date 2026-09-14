package scaffold

import "time"

// Built-in template IDs
const (
	BuiltinIDGoMicroservice = "tpl-builtin-go-microservice"
	BuiltinIDNodeFastify    = "tpl-builtin-node-fastify"
	BuiltinIDPythonFastAPI  = "tpl-builtin-python-fastapi"
	BuiltinIDNginxProxy     = "tpl-builtin-nginx-proxy"
	BuiltinIDPostgresDB     = "tpl-builtin-postgres-db"

	// Backward compatibility aliases
	BuiltinIDGoAPI   = BuiltinIDGoMicroservice
	BuiltinIDNodeWeb = BuiltinIDNodeFastify
)

var builtinSeedTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// GetBuiltinTemplates returns the system-provided starter templates.
func GetBuiltinTemplates() []Template {
	return []Template{
		buildGoMicroserviceTemplate(),
		buildNodeFastifyTemplate(),
		buildPythonFastAPITemplate(),
		buildNginxProxyTemplate(),
		buildPostgresDBTemplate(),
	}
}
