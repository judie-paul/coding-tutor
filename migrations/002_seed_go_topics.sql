-- Seed the Go language and its learning path.
-- New languages can be added later with a new migration following this pattern.

INSERT INTO programming_languages (slug, name, description, created_at, updated_at)
VALUES ('go', 'Go', 'A simple, fast, statically typed language built for reliable software.',
        '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');

INSERT INTO topics (language_id, slug, title, summary, key_points, difficulty, position, created_at, updated_at)
SELECT l.id, t.slug, t.title, t.summary, t.key_points, t.difficulty, t.position,
       '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
FROM programming_languages l
JOIN (
    SELECT 'basics' AS slug, 'Go Basics' AS title,
           'Packages, the main function, imports and how a Go program is structured and run.' AS summary,
           'Every Go file belongs to a package
Programs start in func main() of package main
Imports bring in other packages
go run compiles and runs; go build produces a binary' AS key_points,
           'beginner' AS difficulty, 1 AS position
    UNION ALL SELECT 'variables-types', 'Variables and Types',
           'Declaring variables, basic types, zero values, constants and type conversion.',
           'var and := declare variables
Every type has a zero value
Constants are declared with const
Conversions are always explicit, e.g. float64(x)',
           'beginner', 2
    UNION ALL SELECT 'control-flow', 'Control Flow',
           'if, for and switch: making decisions and repeating work.',
           'for is the only loop keyword
if can start with a short statement
switch cases do not fall through by default
defer runs a call when the function returns',
           'beginner', 3
    UNION ALL SELECT 'functions', 'Functions',
           'Defining functions, multiple return values, variadic functions and closures.',
           'Functions can return several values
Named results are allowed but use them sparingly
Functions are values and can be passed around
Closures capture variables from their surrounding scope',
           'beginner', 4
    UNION ALL SELECT 'slices-arrays', 'Arrays and Slices',
           'Fixed-size arrays, flexible slices, append, len, cap and slicing.',
           'Arrays have a fixed length that is part of their type
Slices are views over an underlying array
append may allocate a new array
Use make to create a slice with a given length and capacity',
           'beginner', 5
    UNION ALL SELECT 'maps', 'Maps',
           'Key/value storage with maps: creating, reading, updating, deleting and iterating.',
           'Create maps with make or a literal
The comma-ok idiom checks whether a key exists
delete removes a key
Map iteration order is not guaranteed',
           'beginner', 6
    UNION ALL SELECT 'structs', 'Structs',
           'Grouping data with structs, struct literals, embedding and pointers to structs.',
           'Structs group related fields
Fields starting with a capital letter are exported
Embedding promotes fields and methods
Use & to take a pointer to a struct literal',
           'beginner', 7
    UNION ALL SELECT 'pointers', 'Pointers',
           'What pointers are, when to use them and how Go passes values.',
           'Go passes everything by value
A pointer holds the address of a value
* dereferences and & takes an address
nil is the zero value of a pointer',
           'intermediate', 8
    UNION ALL SELECT 'methods-interfaces', 'Methods and Interfaces',
           'Attaching methods to types and describing behavior with interfaces.',
           'Methods have a receiver
Pointer receivers can modify the value
Interfaces are satisfied implicitly
Small interfaces like io.Reader are idiomatic',
           'intermediate', 9
    UNION ALL SELECT 'errors', 'Error Handling',
           'Returning and checking errors, wrapping with %w, errors.Is and errors.As.',
           'Errors are ordinary values
Check errors right after the call that returns them
Wrap errors with fmt.Errorf and %w
Use errors.Is and errors.As to inspect wrapped errors',
           'intermediate', 10
    UNION ALL SELECT 'packages-modules', 'Packages and Modules',
           'Organizing code into packages and managing dependencies with Go modules.',
           'A module is defined by go.mod
Each directory is one package
Exported names start with a capital letter
go mod tidy keeps dependencies in sync',
           'intermediate', 11
    UNION ALL SELECT 'testing', 'Testing',
           'Writing tests with the testing package, table-driven tests and benchmarks.',
           'Test files end in _test.go
Test functions look like func TestXxx(t *testing.T)
Table-driven tests cover many cases cleanly
go test ./... runs every test',
           'intermediate', 12
    UNION ALL SELECT 'generics', 'Generics',
           'Type parameters, constraints and writing reusable generic functions and types.',
           'Type parameters go in square brackets
Constraints limit which types are allowed
any and comparable are built-in constraints
Prefer generics only when they remove real duplication',
           'advanced', 13
    UNION ALL SELECT 'goroutines', 'Goroutines',
           'Running work concurrently with goroutines and waiting for it with sync.WaitGroup.',
           'go f() starts a goroutine
Goroutines are cheap but not free
sync.WaitGroup waits for a group of goroutines
Data races happen when goroutines share memory unsafely',
           'advanced', 14
    UNION ALL SELECT 'channels', 'Channels and Select',
           'Communicating between goroutines with channels, buffering and select.',
           'Channels pass values between goroutines
Unbuffered channels synchronize sender and receiver
Closing a channel signals no more values
select waits on several channel operations',
           'advanced', 15
    UNION ALL SELECT 'context', 'Context and Cancellation',
           'Using context.Context for deadlines, cancellation and request-scoped values.',
           'Pass ctx as the first parameter
Always call the cancel function
ctx.Done() is closed when work should stop
Do not store contexts in structs',
           'advanced', 16
) t
WHERE l.slug = 'go';
