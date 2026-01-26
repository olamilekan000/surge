package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/olamilekan000/surge/surge/errors"
	"github.com/olamilekan000/surge/surge/job"
)

type RedisConfig struct {
	URL              string
	Host             string
	Port             int
	DB               int
	Password         string
	Username         string
	PoolSize         int
	MaxRetries       int
	ConnMaxIdleTime  time.Duration
	RecoveryInterval time.Duration
	RecoveryTimeout  time.Duration
	PingTimeout      time.Duration
}

type RedisBackend struct {
	client           *redis.Client
	prefix           string
	recoveryInterval time.Duration
	recoveryTimeout  time.Duration
	cancelRecovery   context.CancelFunc
	recoveryWG       sync.WaitGroup
}

func NewRedisBackend(ctx context.Context, cfg RedisConfig) (*RedisBackend, error) {
	opts := &redis.Options{
		Addr:            fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:        cfg.Password,
		Username:        cfg.Username,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MaxRetries:      cfg.MaxRetries,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	}

	client := redis.NewClient(opts)

	pingTimeout := cfg.PingTimeout
	if pingTimeout == 0 {
		pingTimeout = 5 * time.Second
	}

	redisCtx, cancelRedisCtx := context.WithTimeout(ctx, pingTimeout)
	defer cancelRedisCtx()

	if err := client.Ping(redisCtx).Err(); err != nil {
		client.Close()
		return nil, &errors.BackendConnectionError{
			Backend: "Redis",
			Err:     err,
		}
	}

	recoveryCtx, cancelRecovery := context.WithCancel(ctx)

	backend := &RedisBackend{
		client:           client,
		prefix:           "surge",
		recoveryInterval: cfg.RecoveryInterval,
		recoveryTimeout:  cfg.RecoveryTimeout,
		cancelRecovery:   cancelRecovery,
	}

	if cfg.RecoveryInterval > 0 {
		log.Printf("Starting recovery process with interval %v", cfg.RecoveryInterval)
		backend.recoveryWG.Add(1)
		go func() {
			defer backend.recoveryWG.Done()
			backend.startRecoveryProcess(recoveryCtx)
		}()
	}

	return backend, nil
}

func (r *RedisBackend) scheduledKey() string {
	return fmt.Sprintf("%s:scheduled", r.prefix)
}

var enqueueCmd = redis.NewScript(`
	local key = KEYS[1]
	local nsRegistry = KEYS[2]
	local qRegistry = KEYS[3]
	local pauseKey = KEYS[4]
	
	local data = ARGV[1]
	local priority = ARGV[2]
	local ns = ARGV[3]
	
	if redis.call("EXISTS", pauseKey) == 1 then
		return -1
	end

	if #KEYS == 5 then
		local uniqueKey = KEYS[5]
		local ttl = ARGV[4]
		if redis.call("SET", uniqueKey, "1", "NX", "EX", ttl) ~= false then
		else
			return 2
		end
	end

	redis.call("ZADD", key, priority, data)
	redis.call("SADD", nsRegistry, ns)
	redis.call("SADD", qRegistry, key)
	return 1
`)

func (r *RedisBackend) Push(ctx context.Context, job *job.JobEnvelope) error {
	data, err := json.Marshal(job)
	if err != nil {
		return &errors.BackendOperationError{Operation: "Push", Err: err}
	}

	key := r.queueKey(job.Namespace, job.Queue)
	registryKey := fmt.Sprintf("%s:namespaces", r.prefix)
	queueRegistryKey := fmt.Sprintf("%s:queues", r.prefix)
	pauseKey := r.pauseKey(job.Namespace, job.Queue)

	keys := []string{key, registryKey, queueRegistryKey, pauseKey}
	args := []interface{}{data, float64(job.Priority), job.Namespace}

	if job.UniqueKey != "" {
		keys = append(keys, fmt.Sprintf("%s:unique:%s", r.prefix, job.UniqueKey))
		args = append(args, 3600)
	}

	res, err := enqueueCmd.Run(ctx, r.client, keys, args...).Result()
	if err != nil {
		return &errors.BackendOperationError{Operation: "Push", Err: err}
	}

	val := res.(int64)
	if val == -1 {
		return &errors.QueuePausedError{Namespace: job.Namespace, Queue: job.Queue}
	}

	return nil
}

func (r *RedisBackend) PushBatch(ctx context.Context, jobs []*job.JobEnvelope) error {
	if len(jobs) == 0 {
		return nil
	}

	queueGroups := make(map[string][]*job.JobEnvelope)
	namespaces := make(map[string]bool)
	queues := make(map[string]bool)
	queueInfo := make(map[string]struct {
		namespace string
		queue     string
		pauseKey  string
	})

	for _, job := range jobs {
		queueKey := r.queueKey(job.Namespace, job.Queue)
		if _, exists := queueGroups[queueKey]; !exists {
			queueInfo[queueKey] = struct {
				namespace string
				queue     string
				pauseKey  string
			}{
				namespace: job.Namespace,
				queue:     job.Queue,
				pauseKey:  r.pauseKey(job.Namespace, job.Queue),
			}
		}

		queueGroups[queueKey] = append(queueGroups[queueKey], job)
		namespaces[job.Namespace] = true
		queues[queueKey] = true
	}

	if len(queueInfo) > 0 {
		pipe := r.client.Pipeline()
		pauseCmds := make(map[string]*redis.IntCmd)

		for queueKey, info := range queueInfo {
			pauseCmds[queueKey] = pipe.Exists(ctx, info.pauseKey)
		}

		_, err := pipe.Exec(ctx)
		if err != nil {
			return &errors.BackendOperationError{
				Operation: "PushBatch",
				Err:       err,
			}
		}

		for queueKey, cmd := range pauseCmds {
			exists, err := cmd.Result()
			if err != nil {
				return &errors.BackendOperationError{
					Operation: "PushBatch",
					Err:       err,
				}
			}

			if exists > 0 {
				info := queueInfo[queueKey]
				return &errors.QueuePausedError{
					Namespace: info.namespace,
					Queue:     info.queue,
				}
			}
		}
	}

	pipe := r.client.Pipeline()

	for queueKey, queueJobs := range queueGroups {
		for _, job := range queueJobs {
			data, err := json.Marshal(job)
			if err != nil {
				return &errors.BackendOperationError{
					Operation: "PushBatch",
					Err:       err,
				}
			}
			pipe.ZAdd(ctx, queueKey, redis.Z{Score: float64(job.Priority), Member: data})
		}
	}

	registryKey := fmt.Sprintf("%s:namespaces", r.prefix)
	for ns := range namespaces {
		pipe.SAdd(ctx, registryKey, ns)
	}

	queueRegistryKey := fmt.Sprintf("%s:queues", r.prefix)
	for queueKey := range queues {
		pipe.SAdd(ctx, queueRegistryKey, queueKey)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return &errors.BackendOperationError{
			Operation: "PushBatch",
			Err:       err,
		}
	}

	return nil
}

var scheduleCmd = redis.NewScript(`
	redis.call("ZADD", KEYS[1], ARGV[1], ARGV[2])
	return 0
`)

func (r *RedisBackend) Schedule(ctx context.Context, job *job.JobEnvelope, processAt time.Time) error {
	data, err := json.Marshal(job)
	if err != nil {
		return &errors.BackendOperationError{
			Operation: "Schedule",
			Err:       err,
		}
	}

	timestamp := float64(processAt.UnixNano()) / 1e9

	_, err = scheduleCmd.Run(ctx, r.client, []string{r.scheduledKey()}, timestamp, data).Result()
	if err != nil {
		return &errors.BackendOperationError{
			Operation: "Schedule",
			Err:       err,
		}
	}

	return nil
}

func (r *RedisBackend) GetScheduledJobs(ctx context.Context, namespace, queue string, offset, limit int) ([]*job.JobEnvelope, int64, error) {
	results, err := r.client.ZRangeWithScores(ctx, r.scheduledKey(), 0, 1000).Result()
	if err != nil {
		return nil, 0, &errors.BackendOperationError{Operation: "GetScheduledJobs", Err: err}
	}

	var matches []*job.JobEnvelope
	for _, z := range results {
		var envelope job.JobEnvelope
		strData, ok := z.Member.(string)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(strData), &envelope); err != nil {
			continue
		}

		if envelope.Namespace == namespace && envelope.Queue == queue {
			seconds := int64(z.Score)
			t := time.Unix(seconds, 0)
			envelope.ScheduledAt = &t
			matches = append(matches, &envelope)
		}
	}

	total := int64(len(matches))

	if offset >= len(matches) {
		return []*job.JobEnvelope{}, total, nil
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}

	return matches[offset:end], total, nil
}

var dispatchCmd = redis.NewScript(`
	local jobs = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, ARGV[2])
	if #jobs > 0 then
		redis.call("ZREM", KEYS[1], unpack(jobs))
	end
	return jobs
`)

func (r *RedisBackend) DispatchScheduledJobs(ctx context.Context, limit int) (int, error) {
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := dispatchCmd.Run(ctx, r.client, []string{r.scheduledKey()}, now, limit).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, &errors.BackendOperationError{
			Operation: "DispatchScheduledJobs",
			Err:       err,
		}
	}

	jobs, ok := res.([]interface{})
	if !ok {
		return 0, nil
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	var envelopes []*job.JobEnvelope
	for _, j := range jobs {
		jobData, ok := j.(string)
		if !ok {
			continue
		}

		var envelope job.JobEnvelope
		if err := json.Unmarshal([]byte(jobData), &envelope); err != nil {
			continue
		}
		envelopes = append(envelopes, &envelope)
	}

	if len(envelopes) > 0 {
		if err := r.PushBatch(ctx, envelopes); err != nil {
			return 0, err
		}
	}

	return len(envelopes), nil
}

var popCmd = redis.NewScript(`
	for _, key in ipairs(KEYS) do
		local res = redis.call("ZPOPMAX", key)
		if #res > 0 then
			local jobData = res[1]
			local job = cjson.decode(jobData)
			local id = job.id
			local timeout = tonumber(job.timeout)
			if timeout == nil or timeout == 0 then
				timeout = 600
			end
			
			local now = redis.call("TIME")[1]
			local expireAt = now + timeout
			
			local processingKey = string.gsub(key, ":queue:", ":processing:")
			redis.call("ZADD", processingKey, expireAt, id)
			
			local processingHashKey = processingKey .. ":job:" .. id
			redis.call("HSET", processingHashKey, "data", jobData)
			
			return jobData
		end
	end
	return nil
`)

func (r *RedisBackend) Pop(ctx context.Context, queues []string, timeout time.Duration) (*job.JobEnvelope, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, nil
		}

		activeQueues := make([]string, 0, len(queues))
		for _, queueKey := range queues {
			namespace, queue, err := r.parseQueueKey(queueKey)
			if err != nil {
				continue
			}
			paused, err := r.IsPaused(ctx, namespace, queue)
			if err == nil && !paused {
				activeQueues = append(activeQueues, queueKey)
			}
		}

		if len(activeQueues) > 0 {
			res, err := popCmd.Run(ctx, r.client, activeQueues).Result()
			if err != nil && err != redis.Nil {
				return nil, &errors.BackendOperationError{
					Operation: "Pop",
					Err:       err,
				}
			}

			if res != nil {
				jobData, ok := res.(string)
				if !ok {
					return nil, &errors.BackendOperationError{
						Operation: "Pop",
						Err:       fmt.Errorf("unexpected return type from Lua script"),
					}
				}

				var envelope job.JobEnvelope
				if err := json.Unmarshal([]byte(jobData), &envelope); err != nil {
					return nil, &errors.BackendOperationError{
						Operation: "Pop",
						Err:       err,
					}
				}
				return &envelope, nil
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
			continue
		}
	}
}

var ackCmd = redis.NewScript(`
	local processingKey = KEYS[1]
	local hashKey = KEYS[2]
	local statsKey = KEYS[3]
	local id = ARGV[1]

	if redis.call("ZREM", processingKey, id) == 1 then
		redis.call("DEL", hashKey)
		if statsKey then
			redis.call("INCR", statsKey)
		end
		return 1
	end
	return 0
`)

var retryCmd = redis.NewScript(`
	local processingKey = KEYS[1]
	local queueKey = KEYS[2]
	local hashKey = KEYS[3]
	local dlqKey = KEYS[4]
	local id = ARGV[1]
	local data = ARGV[2]
	local priority = ARGV[3]

	local removed = redis.call("ZREM", processingKey, id)
	local removedDLQ = redis.call("LREM", dlqKey, 1, data)

	if removed == 1 or removedDLQ > 0 then
		redis.call("DEL", hashKey)
		redis.call("ZADD", queueKey, priority, data)
		return 1
	end
	return 0
`)

var scheduleRetryCmd = redis.NewScript(`
	local processingKey = KEYS[1]
	local scheduledKey = KEYS[2]
	local hashKey = KEYS[3]
	local dlqKey = KEYS[4]
	local id = ARGV[1]
	local data = ARGV[2]
	local score = ARGV[3]

	local removed = redis.call("ZREM", processingKey, id)
	local removedDLQ = redis.call("LREM", dlqKey, 1, data)

	if removed == 1 or removedDLQ > 0 then
		redis.call("DEL", hashKey)
		redis.call("ZADD", scheduledKey, score, data)
		return 1
	end
	return 0
`)

var dlqCmd = redis.NewScript(`
	local processingKey = KEYS[1]
	local dlqKey = KEYS[2]
	local hashKey = KEYS[3]
	local indexKey = KEYS[4]
	local id = ARGV[1]
	local data = ARGV[2]
	local namespace = ARGV[3]
	local queue = ARGV[4]

	redis.call("ZREM", processingKey, id)
	redis.call("DEL", hashKey)
	redis.call("LPUSH", dlqKey, data)
	redis.call("HSET", indexKey, id, namespace .. ":" .. queue)
	return 1
`)

func (r *RedisBackend) Ack(ctx context.Context, envelope *job.JobEnvelope) error {
	processingKey := r.processingKey(envelope.Namespace, envelope.Queue)
	processingHashKey := fmt.Sprintf("%s:job:%s", processingKey, envelope.ID)
	statsKey := fmt.Sprintf("%s:processed", r.queueKey(envelope.Namespace, envelope.Queue))

	res, err := ackCmd.Run(ctx, r.client, []string{processingKey, processingHashKey, statsKey}, envelope.ID).Result()
	if err != nil {
		return &errors.BackendOperationError{Operation: "Ack", Err: err}
	}

	if res.(int64) == 0 {
		log.Printf("Warning: Ack failed for job %s (lock lost/expired)", envelope.ID)
	}
	return nil
}

func (r *RedisBackend) Nack(ctx context.Context, envelope *job.JobEnvelope, reason error) error {
	processingKey := r.processingKey(envelope.Namespace, envelope.Queue)
	processingHashKey := fmt.Sprintf("%s:job:%s", processingKey, envelope.ID)
	queueKey := r.queueKey(envelope.Namespace, envelope.Queue)
	dlqKey := r.dlqKey(envelope.Namespace, envelope.Queue)

	envelope.LastError = reason.Error()
	envelope.RetryCount++
	envelope.State = job.StatePending

	if envelope.RetryCount <= envelope.MaxRetries {
		backoffSeconds := 1 << envelope.RetryCount
		if backoffSeconds > 300 {
			backoffSeconds = 300
		}
		nextRetryAt := time.Now().Add(time.Duration(backoffSeconds) * time.Second)
		envelope.NextRetryAt = &nextRetryAt

		jobData, err := json.Marshal(envelope)
		if err != nil {
			return &errors.BackendOperationError{Operation: "Nack", Err: err}
		}

		retryPriority := envelope.Priority - job.Priority(envelope.RetryCount)
		if retryPriority < job.PriorityLow {
			retryPriority = job.PriorityLow
		}
		if retryPriority > job.PriorityCritical {
			retryPriority = job.PriorityCritical
		}

		if backoffSeconds > 0 {
			timestamp := float64(nextRetryAt.UnixNano()) / 1e9

			originalPriority := envelope.Priority
			envelope.Priority = retryPriority

			jobData, err := json.Marshal(envelope)
			if err != nil {
				return &errors.BackendOperationError{Operation: "Nack", Err: err}
			}

			envelope.Priority = originalPriority

			_, err = scheduleRetryCmd.Run(ctx, r.client,
				[]string{processingKey, r.scheduledKey(), processingHashKey, dlqKey},
				envelope.ID, jobData, timestamp).Result()

			if err != nil {
				return &errors.BackendOperationError{Operation: "Nack", Err: err}
			}
		} else {
			_, err = retryCmd.Run(ctx, r.client,
				[]string{processingKey, queueKey, processingHashKey, dlqKey},
				envelope.ID, jobData, float64(retryPriority)).Result()

			if err != nil {
				return &errors.BackendOperationError{Operation: "Nack", Err: err}
			}
		}
		return nil
	}

	return r.MoveToDLQ(ctx, envelope)
}

func (r *RedisBackend) Retry(ctx context.Context, envelope *job.JobEnvelope) error {
	processingKey := r.processingKey(envelope.Namespace, envelope.Queue)
	processingHashKey := fmt.Sprintf("%s:job:%s", processingKey, envelope.ID)
	queueKey := r.queueKey(envelope.Namespace, envelope.Queue)
	dlqKey := r.dlqKey(envelope.Namespace, envelope.Queue)

	envelope.State = job.StatePending
	envelope.LastError = ""
	envelope.NextRetryAt = nil

	jobData, err := json.Marshal(envelope)
	if err != nil {
		return &errors.BackendOperationError{Operation: "Retry", Err: err}
	}

	_, err = retryCmd.Run(ctx, r.client,
		[]string{processingKey, queueKey, processingHashKey, dlqKey},
		envelope.ID, jobData, float64(envelope.Priority)).Result()

	if err != nil {
		return &errors.BackendOperationError{Operation: "Retry", Err: err}
	}

	return nil
}

func (r *RedisBackend) MoveToDLQ(ctx context.Context, envelope *job.JobEnvelope) error {
	envelope.State = job.StateDead

	jobData, err := json.Marshal(envelope)
	if err != nil {
		return &errors.BackendOperationError{Operation: "MoveToDLQ", Err: err}
	}

	processingKey := r.processingKey(envelope.Namespace, envelope.Queue)
	processingHashKey := fmt.Sprintf("%s:job:%s", processingKey, envelope.ID)
	dlqKey := r.dlqKey(envelope.Namespace, envelope.Queue)
	dlqIndexKey := r.dlqIndexKey()

	_, err = dlqCmd.Run(ctx, r.client,
		[]string{processingKey, dlqKey, processingHashKey, dlqIndexKey},
		envelope.ID, jobData, envelope.Namespace, envelope.Queue).Result()

	if err != nil {
		return &errors.BackendOperationError{Operation: "MoveToDLQ", Err: err}
	}

	return nil
}

func (r *RedisBackend) Pause(ctx context.Context, namespace, queue string) error {
	key := r.pauseKey(namespace, queue)
	err := r.client.Set(ctx, key, "1", 0).Err()
	if err != nil {
		return &errors.BackendOperationError{
			Operation: "Pause",
			Err:       err,
		}
	}
	return nil
}

func (r *RedisBackend) Resume(ctx context.Context, namespace, queue string) error {
	key := r.pauseKey(namespace, queue)
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return &errors.BackendOperationError{
			Operation: "Resume",
			Err:       err,
		}
	}
	return nil
}

func (r *RedisBackend) Drain(ctx context.Context, namespace, queue string) (int64, error) {
	queueKey := r.queueKey(namespace, queue)

	count, err := r.client.ZCard(ctx, queueKey).Result()
	if err != nil {
		return 0, &errors.BackendOperationError{
			Operation: "Drain",
			Err:       err,
		}
	}

	err = r.client.ZRemRangeByRank(ctx, queueKey, 0, -1).Err()
	if err != nil {
		return 0, &errors.BackendOperationError{
			Operation: "Drain",
			Err:       err,
		}
	}

	return count, nil
}

func (r *RedisBackend) IsPaused(ctx context.Context, namespace, queue string) (bool, error) {
	key := r.pauseKey(namespace, queue)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, &errors.BackendOperationError{
			Operation: "IsPaused",
			Err:       err,
		}
	}

	return exists > 0, nil
}

func (r *RedisBackend) DiscoverQueues(ctx context.Context) ([]string, error) {
	queueRegistryKey := fmt.Sprintf("%s:queues", r.prefix)
	allQueueKeys, err := r.client.SMembers(ctx, queueRegistryKey).Result()
	if err != nil {
		return nil, &errors.BackendOperationError{
			Operation: "DiscoverQueues",
			Err:       err,
		}
	}

	return allQueueKeys, nil
}

func (r *RedisBackend) QueueStats(ctx context.Context, namespace, queue string) (*QueueStats, error) {
	queueKey := r.queueKey(namespace, queue)
	processingKey := r.processingKey(namespace, queue)
	dlqKey := r.dlqKey(namespace, queue)
	statsKey := fmt.Sprintf("%s:processed", queueKey)
	pauseKey := r.pauseKey(namespace, queue)

	pipe := r.client.Pipeline()
	pendingCmd := pipe.ZCard(ctx, queueKey)
	processingCmd := pipe.ZCard(ctx, processingKey)
	failedCmd := pipe.LLen(ctx, dlqKey)
	processedCmd := pipe.Get(ctx, statsKey)
	pausedCmd := pipe.Exists(ctx, pauseKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, &errors.BackendOperationError{Operation: "QueueStats", Err: err}
	}

	processed, _ := processedCmd.Int64()
	isPaused := pausedCmd.Val() > 0

	return &QueueStats{
		Pending:    pendingCmd.Val(),
		Processing: processingCmd.Val(),
		Failed:     failedCmd.Val(),
		Dead:       0,
		Processed:  processed,
		Paused:     isPaused,
	}, nil
}

func (r *RedisBackend) InspectDLQ(ctx context.Context, namespace, queue string, offset, limit int) ([]*job.JobEnvelope, error) {
	dlqKey := r.dlqKey(namespace, queue)

	start := int64(offset)
	stop := int64(offset + limit - 1)

	rawJobs, err := r.client.LRange(ctx, dlqKey, start, stop).Result()
	if err != nil {
		return nil, &errors.BackendOperationError{Operation: "InspectDLQ", Err: err}
	}

	envelopes := make([]*job.JobEnvelope, 0, len(rawJobs))
	for _, raw := range rawJobs {
		var env job.JobEnvelope
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			log.Printf("Failed to unmarshal DLQ job: %v", err)
			continue
		}
		envelopes = append(envelopes, &env)
	}

	return envelopes, nil
}

var retryFromDLQCmd = redis.NewScript(`
	local dlqKey = KEYS[1]
	local queueKey = KEYS[2]
	local indexKey = KEYS[3]
	local targetId = ARGV[1]

	local jobs = redis.call("LRANGE", dlqKey, 0, -1)
	
	for i, jobData in ipairs(jobs) do
		local success, job = pcall(cjson.decode, jobData)
		if success and job and job.id == targetId then
			redis.call("LREM", dlqKey, 1, jobData)
			local priority = job.priority or 0
			redis.call("ZADD", queueKey, priority, jobData)
			redis.call("HDEL", indexKey, targetId)
			return 1
		end
	end
	
	return 0
`)

func (r *RedisBackend) RetryFromDLQ(ctx context.Context, jobID string) error {
	dlqIndexKey := r.dlqIndexKey()
	location, err := r.client.HGet(ctx, dlqIndexKey, jobID).Result()

	if err == redis.Nil {
		return &errors.JobNotFoundError{JobID: jobID}
	}
	if err != nil {
		return &errors.BackendOperationError{Operation: "RetryFromDLQ", Err: err}
	}

	parts := strings.Split(location, ":")
	if len(parts) != 2 {
		return &errors.BackendOperationError{
			Operation: "RetryFromDLQ",
			Err:       fmt.Errorf("invalid location format in index: %s", location),
		}
	}

	namespace := parts[0]
	queue := parts[1]
	dlqKey := r.dlqKey(namespace, queue)
	queueKey := r.queueKey(namespace, queue)

	res, err := retryFromDLQCmd.Run(ctx, r.client,
		[]string{dlqKey, queueKey, dlqIndexKey},
		jobID).Result()

	if err != nil {
		return &errors.BackendOperationError{Operation: "RetryFromDLQ", Err: err}
	}

	if res.(int64) == 0 {
		return &errors.JobNotFoundError{JobID: jobID}
	}

	return nil
}

var retryAllDLQCmd = redis.NewScript(`
	local dlqKey = KEYS[1]
	local queueKey = KEYS[2]
	local indexKey = KEYS[3]

	local jobs = redis.call("LRANGE", dlqKey, 0, -1)
	local count = 0
	local jobIds = {}

	for i, jobData in ipairs(jobs) do
		local success, job = pcall(cjson.decode, jobData)
		if success and job then
			local priority = job.priority or 0
			redis.call("ZADD", queueKey, priority, jobData)
			table.insert(jobIds, job.id)
			count = count + 1
		end
	end

	if count > 0 then
		redis.call("DEL", dlqKey)
		for i, jobId in ipairs(jobIds) do
			redis.call("HDEL", indexKey, jobId)
		end
	end

	return count
`)

func (r *RedisBackend) RetryAllDLQ(ctx context.Context, namespace, queue string) (int64, error) {
	dlqKey := r.dlqKey(namespace, queue)
	queueKey := r.queueKey(namespace, queue)
	dlqIndexKey := r.dlqIndexKey()

	res, err := retryAllDLQCmd.Run(ctx, r.client,
		[]string{dlqKey, queueKey, dlqIndexKey}).Result()

	if err != nil {
		return 0, &errors.BackendOperationError{Operation: "RetryAllDLQ", Err: err}
	}

	count := res.(int64)
	return count, nil
}

func (r *RedisBackend) CheckUnique(ctx context.Context, uniqueKey string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("%s:unique:%s", r.prefix, uniqueKey)

	result, err := r.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, &errors.BackendOperationError{
			Operation: "CheckUnique",
			Err:       err,
		}
	}

	return result, nil
}

func (r *RedisBackend) GetNamespaces(ctx context.Context) ([]string, error) {
	registryKey := fmt.Sprintf("%s:namespaces", r.prefix)

	namespaces, err := r.client.SMembers(ctx, registryKey).Result()
	if err != nil {
		return nil, &errors.BackendOperationError{
			Operation: "GetNamespaces",
			Err:       err,
		}
	}

	return namespaces, nil
}

func (r *RedisBackend) Close() error {
	if r.cancelRecovery != nil {
		r.cancelRecovery()
		r.recoveryWG.Wait()
	}

	return r.client.Close()
}

func (r *RedisBackend) IsHealthy() bool {
	return r.client.Ping(context.Background()).Err() == nil
}

func (r *RedisBackend) Heartbeat(ctx context.Context, workerID string, ttl time.Duration) error {
	key := fmt.Sprintf("%s:worker:%s", r.prefix, workerID)
	return r.client.Set(ctx, key, 1, ttl).Err()
}

func (r *RedisBackend) startRecoveryProcess(ctx context.Context) {
	log.Printf("Starting recovery process with interval %s and timeout %s", r.recoveryInterval, r.recoveryTimeout)

	ticker := time.NewTicker(r.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.recoverStuckJobs(ctx)
		}
	}
}

func (r *RedisBackend) recoverStuckJobs(ctx context.Context) {
	pattern := fmt.Sprintf("%s:*:processing:*", r.prefix)
	cutoffTime := time.Now().Add(-r.recoveryTimeout).Unix()

	var cursor uint64
	processingKeys := make(map[string]bool)

	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			log.Printf("Failed to scan for processing keys: %v", err)
			return
		}

		for _, key := range keys {
			if !strings.Contains(key, ":job:") {
				processingKeys[key] = true
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	for processingKey := range processingKeys {
		expiredJobs, err := r.client.ZRangeByScore(ctx, processingKey, &redis.ZRangeBy{
			Min: "-inf",
			Max: fmt.Sprintf("%d", cutoffTime),
		}).Result()

		if err != nil {
			log.Printf("Failed to get expired jobs from %s: %v", processingKey, err)
			continue
		}

		for _, jobID := range expiredJobs {
			if err := r.recoverJob(ctx, processingKey, jobID); err != nil {
				log.Printf("Failed to recover job %s: %v", jobID, err)
			}
		}
	}
}

func (r *RedisBackend) recoverJob(ctx context.Context, processingKey, jobID string) error {
	processingHashKey := fmt.Sprintf("%s:job:%s", processingKey, jobID)

	jobData, err := r.client.HGet(ctx, processingHashKey, "data").Result()
	if err == redis.Nil {
		log.Printf("Job %s data lost during processing, moving to DLQ", jobID)
		return r.moveExpiredJobToDLQ(ctx, processingKey, jobID)
	}
	if err != nil {
		return fmt.Errorf("failed to get job data: %w", err)
	}

	var envelope job.JobEnvelope
	if err := json.Unmarshal([]byte(jobData), &envelope); err != nil {
		log.Printf("Failed to unmarshal job %s: %v", jobID, err)
		return r.moveExpiredJobToDLQ(ctx, processingKey, jobID)
	}

	if envelope.RetryCount < envelope.MaxRetries {
		return r.recoverJobToQueue(ctx, &envelope, processingKey)
	}

	return r.MoveToDLQ(ctx, &envelope)
}

var recoverCmd = redis.NewScript(`
	local processingKey = KEYS[1]
	local queueKey = KEYS[2]
	local hashKey = KEYS[3]
	local id = ARGV[1]
	local data = ARGV[2]
	local priority = ARGV[3]

	if redis.call("ZREM", processingKey, id) == 1 then
		redis.call("DEL", hashKey)
		redis.call("ZADD", queueKey, priority, data)
		return 1
	end
	return 0
`)

func (r *RedisBackend) recoverJobToQueue(ctx context.Context, envelope *job.JobEnvelope, processingKey string) error {
	parts := strings.Split(processingKey, ":")
	if len(parts) < 4 {
		return fmt.Errorf("invalid processing key format: %s", processingKey)
	}
	namespace := parts[1]
	queue := parts[3]

	envelope.State = job.StatePending
	envelope.LastError = "Recovered from stuck processing state"
	envelope.RetryCount++

	recoveryPriority := envelope.Priority - job.Priority(envelope.RetryCount)
	if recoveryPriority < job.PriorityLow {
		recoveryPriority = job.PriorityLow
	}

	jobData, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal recovered job: %w", err)
	}

	queueKey := r.queueKey(namespace, queue)
	hashKey := fmt.Sprintf("%s:job:%s", processingKey, envelope.ID)

	res, err := recoverCmd.Run(ctx, r.client,
		[]string{processingKey, queueKey, hashKey},
		envelope.ID, jobData, float64(recoveryPriority)).Result()

	if err != nil {
		return fmt.Errorf("failed to re-queue recovered job: %w", err)
	}

	if res.(int64) == 1 {
		log.Printf("Recovered stuck job %s to queue %s:%s with priority %d", envelope.ID, namespace, queue, recoveryPriority)
		return nil
	}

	log.Printf("Skipped recovery of job %s (no longer in processing state)", envelope.ID)

	return nil
}

func (r *RedisBackend) moveExpiredJobToDLQ(ctx context.Context, processingKey, jobID string) error {
	parts := strings.Split(processingKey, ":")
	if len(parts) < 4 {
		return fmt.Errorf("invalid processing key format: %s", processingKey)
	}
	namespace := parts[1]
	queue := parts[3]

	envelope := &job.JobEnvelope{
		ID:        jobID,
		State:     job.StateDead,
		Namespace: namespace,
		Queue:     queue,
		LastError: "Job data lost due to worker crash during processing",
		CreatedAt: time.Now(),
	}

	jobData, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ job: %w", err)
	}

	dlqKey := r.dlqKey(namespace, queue)
	dlqIndexKey := r.dlqIndexKey()
	processingHashKey := fmt.Sprintf("%s:job:%s", processingKey, jobID)

	_, err = dlqCmd.Run(ctx, r.client,
		[]string{processingKey, dlqKey, processingHashKey, dlqIndexKey},
		jobID, jobData, namespace, queue).Result()

	if err != nil {
		return fmt.Errorf("failed to move expired job to DLQ: %w", err)
	}

	return nil
}

func (r *RedisBackend) queueKey(namespace, queue string) string {
	return fmt.Sprintf("%s:%s:queue:%s", r.prefix, namespace, queue)
}

func (r *RedisBackend) pauseKey(namespace, queue string) string {
	return fmt.Sprintf("%s:%s:paused:%s", r.prefix, namespace, queue)
}

func (r *RedisBackend) processingKey(namespace, queue string) string {
	return fmt.Sprintf("%s:%s:processing:%s", r.prefix, namespace, queue)
}

func (r *RedisBackend) dlqKey(namespace, queue string) string {
	return fmt.Sprintf("%s:%s:dlq:%s", r.prefix, namespace, queue)
}

func (r *RedisBackend) dlqIndexKey() string {
	return fmt.Sprintf("%s:dlq:index", r.prefix)
}

func (r *RedisBackend) parseQueueKey(queueKey string) (namespace, queue string, err error) {
	expectedPrefix := fmt.Sprintf("%s:", r.prefix)
	if !strings.HasPrefix(queueKey, expectedPrefix) {
		return "", "", fmt.Errorf("invalid queue key format: %s", queueKey)
	}

	remaining := queueKey[len(expectedPrefix):]

	parts := strings.Split(remaining, ":")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid queue key format: %s", queueKey)
	}

	if parts[1] != "queue" {
		return "", "", fmt.Errorf("invalid queue key format: expected 'queue' segment, got %s", queueKey)
	}

	namespace = parts[0]
	queue = strings.Join(parts[2:], ":")

	return namespace, queue, nil
}
