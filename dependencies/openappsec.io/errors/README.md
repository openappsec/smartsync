# Errors

This library provides two packages. `errors`, for general error manegement, and `errors/errorLoader`, for loading predefined errors from a JSON file.

## Errors PKG

This package provides an implementation of the golang `error` interface with the addition of error wrapping and classes. 
Error classes mark the type of a given error, while wrapping allows for the creation of error chains. 

For instance, a domain code calling a database operation. 
The database adapter could return a `ClassNotFound` type error with the message "item not found", the domain, having received this error could wrap it with the message "database call failed". 
Furhtermore, by receiving a `ClassNotFound` the domain knows that it has failed due to a missing record in the database and not due to a connection issue for example.

The result would be an error chain with data from both the database adapter and the domain code, each adding their own contextual data to the message. 

The package errorloader reads error from an error file and creates ErrorResponse.

## Operations: 

### New:
creates a new error

### Wrap:
Accepts an error and wraps it within another, adding the given message to the existing error. 

Example:

```go
error1 := errors.New("first message")
error2 := errors.Wrap(error1, "second message")
fmt.Println(error2) // output: "second message: first message"
```
### Wrapf:
Behaves like `Wrap` while also allowing formatting of the given error message

### Errorf:
Behaves like `New` while also allowing formatting of the given error message

### SetClass:
Given and error and a class this function "tags" the error with said class. 

As a result, future calls to `IsClass` will return `true` when checking for said class on the given error. 

**Note:** Calling this func does not change any of the wrapped errors' classes and that errors can be of multiple classes

### IsClass:
Given an error and a class this function checks if this error, or any of the errors wrapped within it, match said class. Returns a boolean value indicating the result. 

Example:

```go
error1 := errors.Errorf("%s message", "first").SetClass(errors.ClassBadInput)
error2 := errors.Wrapf(error1, "%s message", "second").SetClass(errors.ClassInternal)
errors.IsClass(error1, errors.ClassBadInput) // returns true
errors.IsClass(error1, errors.ClassInternal) // returns false
errors.IsClass(error2, errors.ClassInternal) // returns true
errors.IsClass(error2, errors.ClassBadInput) // returns true
```

### IsClassTopLevel:
Given an error and a class this function checks if this error class match input class on top level error, not nested errors, when error class is unknown function will call itself until it find an error class rather then unknown. Returns a boolean value indicating the result. 

Example:

```go
error1 := errors.Errorf("%s message", "first").SetClass(errors.ClassBadInput)
error2 := errors.Wrapf(error1, "%s message", "second").SetClass(errors.ClassInternal)
error3 := errors.Wrapf(error2, "%s message", "third")
errors.IsClassTopLevel(error1, errors.ClassBadInput) // returns true
errors.IsClassTopLevel(error1, errors.ClassInternal) // returns false
errors.IsClassTopLevel(error2, errors.ClassInternal) // returns true
errors.IsClassTopLevel(error2, errors.ClassBadInput) // returns false
errors.IsClassTopLevel(error3, errors.ClassInternal) // returns true
errors.IsClassTopLevel(error3, errors.ClassBadInput) // returns false
```

### SetLabel:
Given and error and a label (any string) this function "labels" the error with said label. 
With the Label it's now possible to distinguish between errors of the same class.

As a result, future calls to `IsLabel` will return `true` when checking for said label on the given error. 

**Note:** Calling this func does not change any of the wrapped errors' labels and that errors can be of multiple labels

### IsLabel:
Given an error and a label this function checks if this error, or any of the errors wrapped within it, match said label. Returns a boolean value indicating the result. 

Example:

```go
error1 := errors.Errorf("%s message", "first").SetLabel("some-label")
error2 := errors.Wrapf(error1, "%s message", "second").SetClass("some-label2")
errors.IsClass(error1, "some-label") // returns true
errors.IsClass(error1, "some-label2") // returns false
errors.IsClass(error2, "some-label2") // returns true
errors.IsClass(error2, "some-label") // returns true
```


### Error:
Implementation of the golang error interface for this package. Returns the message held within the given error. 

## Classes:

| Class Name | Class meaning |
|------------------|-------------------------------------------------------------------------------------|
| `ClassUnknown` | default value when no other class is set |
| `ClassBadInput` | indicates an invalid input by the user |
| `ClassForbidden` | indicates an attempt at a forbidden operation |
| `ClassNotFound` | indicates a failure to find a queried for item |
| `ClassInternal` | indicated a failure of the application itself. Often used when no other class fits. |


## ErrorLoader PKG

### Configure:
Defines the errors file and application code, preparing the package for use. Should always be called upon app initlization if you wish to use the `errorsLoader` pkg.

It first looks for the errors file in working directory, and if not found - looks under the executable directory.

Example errors file:

```json
{
  "default-error": {
    "message": "Unknown Error, contact support",
    "description": "Unknown Error, check logs",
    "code": "000",
    "severity": "High"
  },
  "read-body-error": {
    "message": "Internal Server Error",
    "description": "Failed to read request body",
    "code": "001",
    "severity": "High"
  },
  "internal": {
    "message": "Internal Server Error",
    "description": "Failed to perform business logic, check logs",
    "code": "002",
    "severity": "High"
  }
}
```

Example usage:

```go
errorloader.Configure("error-responses.json", "199")
```

### NewErrorResponse:
Allows creation of a custom `ErrorResponse` instance. This shouldn't relaly be used, instead errors should be loaded from a preconfigured file.

### GetError:
Loads the error with the given key from the configured file. If no such error is found, an `error` (regular golang error) is returned.

Example usage:

```go
errorloader.GetError(ctx, "default-error")
```
