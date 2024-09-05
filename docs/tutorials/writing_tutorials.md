---
title: "Writing executable tutorials"
description: "How to write an executable tutorial"
draft: false
images: []
menu:
  docs:
    parent: "tutorials"
weight: 110
toc: true
---

### The terminology: Stages, chunks and commands

Executable tutorials work a bit like a CI workflow. The command is the smallest
element of them. It corresponds to an executable with its arguments to be called
onto the system. A chunk is a collection of commands and is used to group them
with an environment of execution, chunks can be executed in parallel of each
other within the same stage. And finally, there are the stages, which are a
collection of chunks. Stages gets executed one after the other.

#### Code chunks

In an executable tutorial, every line gets executed. Chunks that are executed are
chunks that provide a valid json object with at least a stage configured in it.

The code fence to create an executable chunk needs to have at least 3 back quotes
and some json metadata on the same line.

````
``` something {"stage":"init"}
```
````

The only mandatory field an executable chunk must have is the stage name.

The other possible fields are:

##### `"runtime":"bash"`

Specify that the chunk is written in bash. The chunk will get turned into a
script and executed.

The script itself is prefixed with `set -euo pipefail`, meaning that it will
error at the first failing commands.

And is suffixed with `printenv` to extract all the environment variables the
user added in.

##### `"runtime":"writer"`

Set that the chunk contains a file to write on disk. The metadata needs to
contain `"destination":"some_place"` to write the file.

##### `"label":"some label"`

You can give a pretty name to a chunk. It's good for bash runtimes, as the
command to execute is always `./script.sh` so if you want to have a
differentiable name in the logs, use this field.

##### `"rootdir":"$operator"`

The commands in the chunk will the executed from the operator source code
directory.


##### `"rootdir":"$tmpdir.x"`

Create a new temporary directory to execute the chunk. If another chunk reuses
the same suffix (`.x` in the example) it'll share the same directory. Useful
when you need to chain chunks or to reuse some cached files between chunks

##### `"parallel":"true"`

Makes the chunk executed in parallel of the others in its stage. This works best
if there's only one command in the chunk (or if the chunk is a bash chunk). And
if all the commands in the stage are made to run in parallel.

For now the parallel behavior has a simplistic implementation. Only the last
command of the chunk is really started asynchronously. The other ones are
sequentially executed before that.

##### `"breakpoint":"true"`

Enters interactive mode when the chunk is started. Useful for debugging
purposes. Better to use alongside `--verbose`

##### `"requires":"stageName/id"`

Makes the execution of the given stage dependant of the correct execution of the
pointed stage. To be used in teardown chunks.

##### `"id":"someID"`

Give an ID to a chunk so that it can get referenced later on


### Execution Environment of the chunks

Every chunk is started with the environment of the parent process that started
it. If as chunk whom runtime is bash is executed, all the variables it adds to
its own env via `export` will get added to the environment of subsequent
chunks.

### Examples

#### Creating and running our first tutorial

Let's create a markdown file containing an executable chunk, and save it as
`simple_tutorial.md`

````md {"stage":"test1", "runtime":"writer", "destination":"simple_tutorial.md", "rootdir":"$tmpdir.1"}
# Demo 1

This is a source markdown file it has a simple chunk that gets executed:

## the chunk
```bash {"stage":"inner_test1"}
echo this line will get printed in the output chunk below.
```
When this document is going to get executed, it'll have a new block above this
line containing the actual output of the executed chunk.
````

We will need to pass the folder containing this markdown file to the tutorial
tester.

```bash {"stage":"test1", "rootdir":"$tmpdir.1", "runtime":"bash"}
export TUTORIALS_FOLDER=$(pwd)
```

Now it's time to run the script.

```bash {"stage":"test1", "runtime":"bash"}
cd ${OPERATOR_ROOT}
go run test/utils/tutorials/tester.go \
   --update-tutorials \
   --quiet \
   --tutorials-root ${TUTORIALS_FOLDER}
```

As we've executed the tutorial tester tool with the `--update-tutorial` option.
It'll make the tool insert the output of the chunk (if any) inside the
source markdown file. Let's have a look at the updated markdown file:

````bash {"stage":"test1", "rootdir":"$tmpdir.1"}
cat simple_tutorial.md
````
````shell tutorial_tester
# Demo 1

This is a source markdown file it has a simple chunk that gets executed:

## the chunk
```bash {"stage":"inner_test1"}
echo this line will get printed in the output chunk below.
```
```shell tutorial_tester
this line will get printed in the output chunk below.
```
When this document is going to get executed, it'll have a new block above this
line containing the actual output of the executed chunk.
````

#### Sharing variables between chunks

Any export in a bash environment is available for subsequent chunks.

````
```bash {"stage":"test2", "runtime":"bash"}
export SOME_VARIABLE=$(sleep .1s && echo "This is some content")
export TEST="some value"
```
````

The chunk below is able to perform some comparisons with value of
`SOME_VARIABLE`

````
```bash {"stage":"test2", "runtime":"bash"}
if [ "$SOME_VARIABLE" == "This is some content" ]; then
  echo "same string"
fi
```
```shell tutorial_tester
same string
```
````

#### Executing in parallel

Parallelism can be important for some workflows, when for instance two processes
need to chat with each other.

To demonstrate the capability, we're executing two pairs of chunks.
The first pair runs in parallel, and does concurrent writing to a file on disk.
The second pair acts as the control group and performs the writing sequentially,
one after the other.

On a parallel environment we expect that the output of the two files should
be different. Let's put that to the test!

##### The first pair is writing to `output` in parallel

````
```bash {"stage":"test3", "runtime":"bash", "parallel":true, "rootdir":"$tmpdir.2"}
for i in $(seq 1 10);
do
    echo FIRST$i >> output
    sleep .1
done
```
````

````
```bash {"stage":"test3", "runtime":"bash", "parallel":true, "rootdir":"$tmpdir.2"}
for i in $(seq 1 10);
do
    sleep .1
    echo SECOND$i >> output
done
```
````

##### The second pair is writing to `output2` sequentially this time

````
```bash {"stage":"test4", "runtime":"bash", "rootdir":"$tmpdir.2"}
for i in $(seq 1 10);
do
    echo FIRST$i >> output2
done
```
````

````
```bash {"stage":"test4", "runtime":"bash", "rootdir":"$tmpdir.2"}
for i in $(seq 1 10);
do
    echo SECOND$i >> output2
done
```
````

##### Then let's compare the two files and asses that the two files are different

````
```{"stage":"test4", "rootdir":"$tmpdir.2", "runtime":"bash"}
DIFF=$(diff output output2 || echo "")
if [[ -z ${DIFF} ]]; then
    echo "the files are the same, that's a problem."
    exit 1
else
    echo "the two files are different, we're running in parallel"
fi
```
```shell tutorial_tester
the two files are different, we're running in parallel
```
````

#### Teardown & dependencies

Teardown chunks are executed even if something went wrong in the middle of the
execution.

Adding a dependency to the teardown chunk makes it execute only if its dependant
chunk did execute correctly. That allows to make sure that a teardown stage only
tears down things that have been built before.

````md {"stage":"test5", "runtime":"writer", "destination":"simple_tutorial.md", "rootdir":"$tmpdir.3"}
# Demo teardown

## Two classical chunks

```bash {"stage":"inner_test1", "id":"someID", "label":"working command"}
echo this executes correctly
```

```bash {"stage":"inner_test1", "id":"someID2", "runtime":"bash", "label":"failing command"}
echo this has failed
exit 1
```

## Two teardown chunks

This one has an output, because it's dependency did run correctly

```bash {"stage":"teardown", "requires":"inner_test1/someID"}
echo executed because test4/someID got executed
```

This one won't

```bash {"stage":"teardown", "requires":"inner_test1/someID2"}
echo not executed because someID2 is missing in test4
```
````
```bash {"stage":"test5", "rootdir":"$tmpdir.3", "runtime":"bash"}
export TUTORIALS_FOLDER=$(pwd)
```

Let's run and see the result

```bash {"stage":"test5", "runtime":"bash"}
cd ${OPERATOR_ROOT}
go run test/utils/tutorials/tester.go \
   --no-styling \
   --tutorials-root ${TUTORIALS_FOLDER} | grep "SUCCESS.*someID" || exit 0
```
```shell tutorial_tester
                                                                                SUCCESS: echo executed because test4/someID got executed

```
As we can see only the first teardown command did run.
