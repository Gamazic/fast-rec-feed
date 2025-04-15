# Fast Feed App

Fast Golang recommendation feed backend application.

# Tutorial

## How to Easily Handle 200k RPS with Golang

Handling 200,000 requests per second (RPS) can truly be considered a high-load system. Achieving such performance often requires hundreds or even thousands of service instances and database shards. However, in this article, I will focus on how a single instance of an application written in Golang can achieve this impressive performance without complicated sharding or replication.

Golang is an excellent choice for high-performance applications. It provides powerful concurrency tools, simplicity, reliability, and speed. The concepts discussed here are broadly applicable to high-performance systems, not limited to Go.

## Example Application

Let's consider a simple recommendation feed service. To simplify, we assume this feed is updated offline. This means the feed is generated once at application startup and never changes during runtime. This pattern is common in recommendation systems to effectively handle high concurrency.

This HTTP-based application has one straightforward goal: return a pre-generated feed for a given user and save progress.

**TL;DR Benchmark Results (MacBook Pro 14, M3, 16GB RAM):**
```
Thread Stats   Avg      Stdev     Max   +/- Stdev
Latency       1.07ms    3.19ms 106.26ms   96.11%
Requests/sec: 211042.29
```

## Business Logic

Let's first define the service's business logic in code and describe struct `FeedService` that contains bussiness logic:

```go
type FeedService struct {
	feedStorage       feedStorage
	randomFeedStorage randomFeedStorage
	errRecorder       errRecorder
	logger            *slog.Logger
}

func NewFeedService(feedStorage feedStorage, randomFeedStorage randomFeedStorage, errRecorder errRecorder, logger *slog.Logger) *FeedService {
	return &FeedService{
		feedStorage:       feedStorage,
		randomFeedStorage: randomFeedStorage,
		errRecorder:       errRecorder,
		logger:            logger,
	}
}

type feedStorage interface {
	GetNextFeed(ctx context.Context, userId uint32, size uint8) ([]uint32, error)
}

type randomFeedStorage interface {
	GetRandomFeed(ctx context.Context, size uint8, excludeItems []uint32) []uint32
}

type errRecorder interface {
	FeedError(ctx context.Context, userId uint32, err error)
}
```

Now, let's implement the method to retrieve the feed:

```go
func (f *FeedService) RetrievFeed(ctx context.Context, r FeedRequest) ([]uint32, error) {
	// Set default size if not specified
	if r.Size == 0 {
		r.Size = defailtNextFeedSize
	}

	var randomFeedSize uint8
	// Get personalized feed for user
	persFeed, err := f.feedStorage.GetNextFeed(ctx, r.UserId, r.Size)
	if err != nil {
		f.errRecorder.FeedError(ctx, r.UserId, err)
	}
	randomFeedSize = r.Size - uint8(len(persFeed))

	// Fill remaining items with random feed
	if randomFeedSize > 0 {
		randomFeed := f.randomFeedStorage.GetRandomFeed(ctx, randomFeedSize, persFeed)
		persFeed = append(persFeed, randomFeed...)
	}

	// Validate final feed size
	if len(persFeed) != int(r.Size) {
		f.errRecorder.FeedError(ctx, r.UserId, fmt.Errorf("feed size is not equal to requested size"))
		f.logger.ErrorContext(ctx, "critical error feed size is not equal to requested size",
			"userId", r.UserId,
			"randomFeedSize", randomFeedSize,
			"persFeedSize", len(persFeed),
			"requestedSize", r.Size)
		if len(persFeed) == 0 {
			return nil, fmt.Errorf("no feed items")
		}
	}

	return persFeed, nil
}
```

To ensure stability, if there aren't enough personalized items (for example, the user has already viewed them or an error occurred), we supplement the feed with random items.
Go's explicit error handling significantly contributes to the reliability of this approach.

At the end, if it's anyway isn't possible to make feed we send error message and return error.


## API

There is a good choice for webserver of HTTP API: `fasthttp`, a high-performance server library known for its **zero allocation** feature. Zero allocation means minimal memory allocation and no unnecessary goroutine spawning per request. Instead, it uses preallocation, buffering, and worker pools to optimize performance.

However, it's can be confusing that request data is only valid within the request's lifecycle. If data needs to persist beyond this, you must explicitly copy it, potentially affecting performance.

Using `fiber`, built on top of `fasthttp`, simplifies working with HTTP requests handling:

```go
type App struct {
	feedService *feed.FeedService
	fiberApp    *fiber.App
}

func NewApp() *App {
	feedService := feed.NewFeedService(...)
	app := &App{
		feedService: feedService,
		fiberApp:    fiber.New(),
	}
	app.fiberApp.Get("/feed/:userId", app.feedHandler)
	return app
}

func (a *App) feedHandler(ctx *fiber.Ctx) error {
	// Get userId from path params
	userId, err := ctx.ParamsInt("userId")
	if err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).SendString("userId is required")
	}
	// Get optional size from query params
	size := ctx.QueryInt("size", 0)

	// Call feed service to get items
	feed, err := a.feedService.RetrievFeed(ctx.Context(), feed.FeedRequest{
		UserId: uint32(userId),
		Size:   uint8(size),
	})
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Format feed items as array string
	var sb strings.Builder
	sb.WriteString("[")
	for i, id := range feed {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatUint(uint64(id), 10))
	}
	sb.WriteString("]")

	// Return response
	return ctx.Status(fiber.StatusOK).SendString(sb.String())
}
```

> This example uses custom serialization, which is highly efficient. In many cases, serialization and deserialization consume significant resources and can slow down the application—especially when working with JSON that contains many fields. To improve performance, you can optimize JSON handling by using high-performance libraries or code generation tools that create parsers for specific data structures. Or even you can switch to a different message type (other than JSON), for example, using binary formats like protobuf in gRPC or other custom serializers.


## Storage

The fastest way to handle a high volume of requests is to keep the data in memory. This avoids the overhead of querying external caches or databases—everything is already available in RAM, and we just need to access it directly.

Below is the implementation of the `feedStorage` used by the `FeedService`:

```go
type Storage struct {
	feeds     map[uint32][feed.TotalFeedSize]uint32
	offsets   sync.Map
	numExceed atomic.Uint64
}

func NewStorage() *Storage {
	return &Storage{
		feeds: make(map[uint32][feed.TotalFeedSize]uint32),
	}
}

func (s *Storage) GetNextFeed(ctx context.Context, userId uint32, size uint8) ([]uint32, error) {
	// Retrieve current offset for the user
	offsetVal, _ := s.offsets.Load(userId)
	var offset uint16
	if offsetVal != nil {
		offset = offsetVal.(uint16)
	}

	// Return empty if the user has already seen all items
	if int(offset) >= feed.TotalFeedSize {
		return nil, nil
	}

	// Calculate the range of items to return
	lastItem := min(int(offset)+int(size), feed.TotalFeedSize)
	if lastItem >= feed.TotalFeedSize {
		s.numExceed.Add(1)
	}

	// Fetch the user's feed and extract the requested slice
	feed, ok := s.feeds[userId]
	if !ok {
		return nil, fmt.Errorf("no feed found for user %d", userId)
	}
	items := feed[offset:lastItem]

	// Update the user's offset
	s.offsets.Store(userId, uint16(lastItem))
	return items, nil
}
```

For in-memory storage, we use two types of hash maps: Go’s built-in `map` and `sync.Map`. The main performance benefit of a hash map is its ability to access random elements in constant time (O(1)), making it ideal for retrieving data by key.

However, regular maps are not safe for concurrent read-write operations and can lead to race conditions and panics. To safely perform concurrent access, there are two main strategies:
	1.	Protect the `map` with a `sync.RWMutex` to lock it during writes.
	2.	Use a lock-free structure like `sync.Map`.

The best choice depends on the type and frequency of access. In highly concurrent environments, `sync.Map` often performs better due to its lock-free behavior.

In our case:
* For storing the static array of video IDs, we use a regular `map`, since it’s read-only during runtime.
* For tracking user offsets, we use `sync.Map` because it needs frequent concurrent updates.


## Benchmark Results

Now it's time to generate data for a benchmark. For this test I generated data for 5 million users, each with 200 feed items (around 7GB RAM usage). More I could not afford because of my laptop.

For benchmarking I will use simple tool `wrk`, which will send random requests like `http://localhost:8080/feed/{random 0...5000000}`.

In my testing scenario, sending requests for randomly selected users worked well. This simulates a typical case where users behave similarly and access their feeds at a fairly even rate.

```
wrk -t5 -c200 -d30s --latency -s benchmark/random_ids.lua http://localhost:8080

Running 30s test @ http://localhost:8080
  5 threads and 200 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     1.07ms    3.19ms 106.26ms   96.11%
    Req/Sec    42.49k     9.63k   76.44k    70.86%
  Latency Distribution
     50%  393.00us
     75%  728.00us
     90%    1.91ms
     99%   12.85ms
  6352517 requests in 30.10s, 1.06GB read
  Socket errors: connect 0, read 61, write 0, timeout 0
Requests/sec: 211042.29
Transfer/sec:     36.14MB
```

#### Interpreting the Benchmark

The system was loaded with 5 million users. It successfully handled around 200,000 requests per second (RPS). That means the service can support all 5 million users if, on average, each user requests their feed once every 25 seconds (5,000,000 / 200,000 ≈ 25s). This is a realistic usage pattern for many real-world applications.

Latency is also low: the average request time is just 1 millisecond, and 99% of requests complete in under 12 milliseconds—which feels instant to the user.

By the end of the benchmark test, fewer than 1% of users had exhausted their personalized feed and started receiving random content. This confirms that the system’s high performance isn’t just due to falling back on random data—it’s capable of delivering personalized results at scale.


## Conclusion

You can find the full example here: https://github.com/Gamazic/fast-rec-feed

My goal with this article was to show how you can build a high-performance system in Go, based on a near real-world use case. Similar architectures are used in many high-load systems — like advertising platforms, trading engines, recommendation services, and search engines.

This example demonstrates several key patterns that enable such performance. While I didn’t cover every aspect of high-load system design, this implementation follows some core principles and can serve as a practical starting point for learning how to build scalable, fast applications.

In future articles, I plan to go deeper into specific high-performance patterns and provide more hands-on examples. Follow me if you’re interested.

And if you’re looking to collaborate — I’m open to new opportunities:

* 📧 Email: nikita.nov.ru@gmail.com
* 💼 LinkedIn: linkedin.com/in/nikita-burov
* 🧵 Reddit: u/EasyButterscotch1597
* 💻 GitHub: github.com/Gamazic
