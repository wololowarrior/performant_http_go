### APIs

1. The http server has with couple of endpoints
2. `/api/verve/accept` accepts `id` and `endpoint` as query params. `id` being mandatory.
3. `/test` this `POST` endpoint is called from the above.

### DeDuplication

1. By using a thread-safe cache `sync.Map` to store the `id` we can prevent same request being counted more than once.
2. When the request lands to onto the handler `accept`, we check if the `id` exists in the map
   1. If not then proceed with logic
   2. Fail the request
3. Injected in the `RequestsHandler` is a `Cache` interface. Currently I've used an inmemory cache `sync.Map`. But since in `extension-1` we can have multiple instances of the app we need to use Redis to cache what we've processed in the minute. 
4. To do that we just need to write three methods on Redis struct and pass it to our handler.

#### Logic around dedeuplication

1. Since the throughput of the system is very high, its not possible to clear the cache every minute.
   1. It'll be very inefficient as there can be 50k+ unique objects per minute.
   2. If we use TTL on objects we have to take care of setting a proper TTL to expire the object at the end of the minute,
   can make system unstable as we have to compute the exact moment we expire the entry.
2. I add a minute identifier along with the cache key. <minute-id>:<objectid>
   1. The minute starts at `0` and as every minute passes this integer is increased.
   2. Cache the id along with the minute id, frees us from cleaning up the cache at every minute end and we can just go on adding same ids at the start of new minute. Since it won't clash with the old one.
   3. The value being the timestamp at which the entry was created. This is to not evict new and valid entries
  
#### Cache cleanup

1. There is a cleanup thread that runs at configurable intervals (it has a cooldown period), removing objects which were created more than a minute ago.
2. We evict the entries in configurable batches as not to overwhelm the system and only spend time cleaning up the cache.


```
minute x 
Req id1 ... cache = {x:id1}
Req id2 ... cache = {x:id2}
.
.
Req id2 ... exists in cache fail request

minute x+1
<<<<<Cleanup thread>>>>> # removes old object
Req id1 ... cache = {x+1:id1}
.
.

```

## Logging the request per minute

1. We start a ticker during initial phase of the app. Init a RPM counter which is an atomic integer, so that we can increament it without using a lock.
2. And in a go routine when the ticker is fired we dump the RPM in the log file and revert the RPM counter to 0.
3. We also increase the minute-id we talked about in the last section here.

Using streaming service

1. For streaming the RPM count we use kafka.
2. Kafka is deployed as an container and has auto topic creation enabled.
3. Instead of logging to a file we write to the topic.

## Extension-1

The current codebase uses inmemory cache. But to have deduplication work across many instances is relatively simple. 
We can use redis cache instead of the inmemory cache. From the code perspective we just need to inject redis struct with three custom methods.

## Extension-2

The current code steams to kafka service. Logging to the file can be enabled by uncommenting a line