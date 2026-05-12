package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogMapperConfig struct {
	KeyMappings              map[string]string
	DroppedKeysIfTypeMatches map[string]zapcore.FieldType
	Constants                map[string]string
}

type LogMapper struct {
	zapcore.Core
	config *LogMapperConfig
}

func NewLogMapper(core zapcore.Core, config *LogMapperConfig) *LogMapper {
	if config == nil {
		config = DefaultLogMapperConfig()
	}
	return &LogMapper{
		Core:   core,
		config: config,
	}
}

func DefaultLogMapperConfig() *LogMapperConfig {
	return &LogMapperConfig{
		KeyMappings: map[string]string{
			// Controller-runtime standard fields -> ECS fields
			"apiVersion":     "orchestrator.api_version",
			"controllerKind": "orchestrator.resource.type", // Resource kind (e.g., "Organization")
			"namespace":      "orchestrator.namespace",     // Resource Kubernetes namespace
			"name":           "orchestrator.resource.name", // Resource name
			"reconcileID":    "trace.id",                   // Reconciliation ID

			// GitHub-specific custom fields
			"github.organization": "custom.github.organization",
			"github.repository":   "custom.github.repository",
			"github.team":         "custom.github.team",
		},
		DroppedKeysIfTypeMatches: map[string]zapcore.FieldType{
			"controller":      zapcore.StringType,
			"controllerGroup": zapcore.StringType,
			"ecs.version":     zapcore.StringType, // drop happens before constants
			"Repository":      zapcore.ReflectType,
			"Team":            zapcore.ReflectType,
			"Organization":    zapcore.ReflectType,
		},
		Constants: map[string]string{
			"ecs.version": "9.3.0", // orchestrator fields are part of 9.3.0
		},
	}
}

func (c *LogMapper) With(fields []zapcore.Field) zapcore.Core {
	remappedFields := c.remapFields(fields)
	return &LogMapper{
		Core:   c.Core.With(remappedFields),
		config: c.config,
	}
}

func (c *LogMapper) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *LogMapper) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	newFields := c.remapFields(fields)
	for k, v := range c.config.Constants {
		newFields = append(newFields, zap.String(k, v))
	}
	return c.Core.Write(ent, newFields)
}

func (c *LogMapper) remapFields(fields []zapcore.Field) []zapcore.Field {
	remappedFields := make([]zapcore.Field, 0)
	for _, field := range fields {
		if fieldType, shouldDrop := c.config.DroppedKeysIfTypeMatches[field.Key]; shouldDrop && field.Type == fieldType {
			continue // Skip (= drop) this field entirely
		}
		if newKey, exists := c.config.KeyMappings[field.Key]; exists {
			remappedFields = append(remappedFields, zapcore.Field{
				Key:       newKey,
				Type:      field.Type,
				Integer:   field.Integer,
				String:    field.String,
				Interface: field.Interface,
			})
		} else {
			remappedFields = append(remappedFields, field)
		}
	}
	return remappedFields
}
