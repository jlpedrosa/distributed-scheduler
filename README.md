# distributed-scheduler

## Intro
A distributed scheduler based on redis, uses multiple sorted sets using time as a score to determine what items
are due and needs to be scheduled. All solution is based on golang.

## Running it
The PoC can be run by using docker compose: `docker compose up --build` additionaly there are helper make targets
```
start-poc:
	@docker compose up --build

cleanup-poc:
	@docker compose down  --rmi local --volumes
```


## Contaiers
### API/Scheduler
Simple API server based on GIN framework, serves two endpoints:

Method to create schedules:
```
POST /ticks
{
    "item": "user1234",
    "at": "2023-02-28T14:16:00Z"
}
```

Method to determine if the service is up (just for container ordering)
```
GET /healthy
```

When a new tick is created, is stored in one of the redis sets. At the due date (spefied by "at" member)
it will emmit a Kafka message, with the same structure in the topic `ticks`.

For convenience, the API service will provision the kafka topic on startup despite this would not be a normal behaviour
on production.

## Consumer
Represents a dummy service that needs to be notified when the ticks are due, so this service subscribes to `tick` topic
and should execute business/domain logic when the events are received. This can be used to receive clocks ticks in an
state machine model, where the clock is represented as a tick event.

## Generator
Is a load generator, creates a list of 1.000.000 different IDs and stress/tests the API, creating random ticks
with random ids from the generated list and scheduling with a random time in the future within 10 minutes.


## understanding the output:

```
docker host                           | Date               | Log message
distributed-scheduler-scheduler-1     | 2023/03/01 02:07:46 Scheduled items/s: 7941
distributed-scheduler-consumer-1      | 2023/03/01 02:07:46 Consumed items/s: 15573
distributed-scheduler-generator-1     | 2023/03/01 02:07:47 Generated ticks/s: 16073
distributed-scheduler-scheduler-2     | 2023/03/01 02:07:45 Scheduled items/s: 7753
```

* Generated items/s: number of http messages sent by the generator container
* Consumed items/s number of kafka messages consumed on the `tick` topic
* Scheduled items/s: number of ticks with expired date that have been pushed to `tick` topic.

In the sample scenario pasted above,
The generator is producing 15.5K rest requests per second.
The scheduler is "bridging" between redis and kafka 15.6K messages per second
The consumer kafka subscription is processing 16K  messages per second.

The numbers between the different stages should be similar, but not exactly the same as the messages are scheduled
in the future with some random delay + the fact that it's an async process.

a very high level overview of the system resources while running the PoC:
```
PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                                                                                                                                                                                                                                                                                                                                                                                                  
20   0 7328392 305760  13676 S 409.6   0.5   6:55.22 scheduler                                                                                                                                                                                                                                                                                                                                                                                                                                                
20   0 3673392 157424   8748 S 251.8   0.2   4:27.94 generator                                                                                                                                                                                                                                                                                                                                                                                                                                                
20   0  829588 146720  31768 S 170.4   0.2   1:17.54 docker-compose                                                                                                                                                                                                                                                                                                                                                                                                                                           
20   0   11.3g   1.0g  20200 S  88.7   1.6   1:30.94 java                                                                                                                                                                                                                                                                                                                                                                                                                                                     
20   0 6962484 286528  13136 S  65.4   0.4   0:53.12 scheduler                                                                                                                                                                                                                                                                                                                                                                                                                                                
20   0  177576  33400   6708 S  41.5   0.1   0:39.18 redis-server                                                                                                                                                                                                                                                                                                                                                                                                                                             
20   0  170408  28504   6772 S  39.9   0.0   0:37.19 redis-server                                                                                                                                                                                                                                                                                                                                                                                                                                             
20   0  160164  27404   6720 S  30.6   0.0   0:29.24 redis-server                                                                                                                                                                                                                                                                                                                                                                                                                                             
20   0 2498604  18856  11028 S  28.6   0.0   0:22.81 consumer 
```