# Ctxutils

This package serves as a utility for context operations and a collection of useful consts. 

## Operations:

### Insert:
Allows the user to add a field into a given context which can be later extracted using the `Extract` operation in this package. 
Given a context, key (`string`), and a value (`interface{}`), the value is set into the context under said key, the returned context is a child of the original given context.

### Extract:
Allows the user to retrive data from a given context.
Given a context and a key (`string`), the context is checked for a value under said key. If a value exists, it is returned (`interface{}`), otherwise a nil value is returned. 

### ExtractString:
Behaves the same way as `Extract` with the added stage of type-assertion of the value into a `string`, returning a `string` value. 

Should the value not exist or the type assertion fail, an empty string is returned. 

### Detach
Detach returns a context that keeps all the values of its parent context
but detaches from the cancellation and error handling.

<b>Note:</b> Only use this if you know what you're doing. This is an advanced feature that goes against typical best practices.

## Consts:
These consts serve as well-defined keys to use with the context operations. For instance setting a value for the key `tenantId`. 

### Known consts:
| Const name | Const value |
|---------------------------|---------------|
| `ContextKeyTenantID`      | tenantId      |
| `ContextKeyTraceID`       | traceId       |
| `ContextKeySourceID`      | sourceId      |
| `ContextKeyAgentID`       | agentId       |
| `ContextKeyProfileID`     | profileId     |
| `ContextKeyCorrelationID` | correlationId |
| `ContextKeyRequestID`     | requestId     |