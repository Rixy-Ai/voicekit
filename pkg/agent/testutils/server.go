package testutils

import (
	"context"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/frostbyte73/core"
	"github.com/gammazero/deque"
	"golang.org/x/exp/maps"

	"github.com/voicekit/voicekit-server/pkg/agent"
	"github.com/voicekit/voicekit-server/pkg/config"
	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/voicekit-server/pkg/service"
	"github.com/voicekit/protocol/auth"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/utils/events"
	"github.com/voicekit/protocol/utils/guid"
	"github.com/voicekit/protocol/utils/must"
	"github.com/voicekit/protocol/utils/options"
	"github.com/voicekit/psrpc"
)

type AgentService interface {
	HandleConnection(context.Context, agent.SignalConn, agent.WorkerRegistration)
	DrainConnections(time.Duration)
}

type TestServer struct {
	AgentService
}

func NewTestServer(bus psrpc.MessageBus) *TestServer {
	localNode, _ := routing.NewLocalNode(nil)
	return NewTestServerWithService(must.Get(service.NewAgentService(
		&config.Config{Region: "test"},
		localNode,
		bus,
		auth.NewSimpleKeyProvider("test", "verysecretsecret"),
	)))
}

func NewTestServerWithService(s AgentService) *TestServer {
	return &TestServer{s}
}

type SimulatedWorkerOptions struct {
	Context            context.Context
	Label              string
	SupportResume      bool
	DefaultJobLoad     float32
	JobLoadThreshold   float32
	DefaultWorkerLoad  float32
	HandleAvailability func(AgentJobRequest)
	HandleAssignment   func(*voicekit.Job) JobLoad
}

type SimulatedWorkerOption func(*SimulatedWorkerOptions)

func WithContext(ctx context.Context) SimulatedWorkerOption {
	return func(o *SimulatedWorkerOptions) {
		o.Context = ctx
	}
}

func WithLabel(label string) SimulatedWorkerOption {
	return func(o *SimulatedWorkerOptions) {
		o.Label = label
	}
}

func WithJobAvailabilityHandler(h func(AgentJobRequest)) SimulatedWorkerOption {
	return func(o *SimulatedWorkerOptions) {
		o.HandleAvailability = h
	}
}

func WithJobAssignmentHandler(h func(*voicekit.Job) JobLoad) SimulatedWorkerOption {
	return func(o *SimulatedWorkerOptions) {
		o.HandleAssignment = h
	}
}

func WithJobLoad(l JobLoad) SimulatedWorkerOption {
	return WithJobAssignmentHandler(func(j *voicekit.Job) JobLoad { return l })
}

func WithDefaultWorkerLoad(load float32) SimulatedWorkerOption {
	return func(o *SimulatedWorkerOptions) {
		o.DefaultWorkerLoad = load
	}
}

func (h *TestServer) SimulateAgentWorker(opts ...SimulatedWorkerOption) *AgentWorker {
	o := &SimulatedWorkerOptions{
		Context:            context.Background(),
		Label:              guid.New("TEST_AGENT_"),
		DefaultJobLoad:     0.1,
		JobLoadThreshold:   0.8,
		DefaultWorkerLoad:  0.0,
		HandleAvailability: func(r AgentJobRequest) { r.Accept() },
		HandleAssignment:   func(j *voicekit.Job) JobLoad { return nil },
	}
	options.Apply(o, opts)

	w := &AgentWorker{
		workerMessages:         make(chan *voicekit.WorkerMessage, 1),
		jobs:                   map[string]*AgentJob{},
		SimulatedWorkerOptions: o,

		RegisterWorkerResponses: events.NewObserverList[*voicekit.RegisterWorkerResponse](),
		AvailabilityRequests:    events.NewObserverList[*voicekit.AvailabilityRequest](),
		JobAssignments:          events.NewObserverList[*voicekit.JobAssignment](),
		JobTerminations:         events.NewObserverList[*voicekit.JobTermination](),
		WorkerPongs:             events.NewObserverList[*voicekit.WorkerPong](),
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())

	if o.DefaultWorkerLoad > 0.0 {
		w.sendStatus()
	}

	ctx := service.WithAPIKey(o.Context, &auth.ClaimGrants{}, "test")
	go h.HandleConnection(ctx, w, agent.MakeWorkerRegistration())

	return w
}

func (h *TestServer) Close() {
	h.DrainConnections(1)
}

var _ agent.SignalConn = (*AgentWorker)(nil)

type JobLoad interface {
	Load() float32
}

type AgentJob struct {
	*voicekit.Job
	JobLoad
}

type AgentJobRequest struct {
	w *AgentWorker
	*voicekit.AvailabilityRequest
}

func (r AgentJobRequest) Accept() {
	identity := guid.New("PI_")
	r.w.SendAvailability(&voicekit.AvailabilityResponse{
		JobId:               r.Job.Id,
		Available:           true,
		SupportsResume:      r.w.SupportResume,
		ParticipantName:     identity,
		ParticipantIdentity: identity,
	})
}

func (r AgentJobRequest) Reject() {
	r.w.SendAvailability(&voicekit.AvailabilityResponse{
		JobId:     r.Job.Id,
		Available: false,
	})
}

type AgentWorker struct {
	*SimulatedWorkerOptions

	fuse           core.Fuse
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	workerMessages chan *voicekit.WorkerMessage
	serverMessages deque.Deque[*voicekit.ServerMessage]
	jobs           map[string]*AgentJob

	RegisterWorkerResponses *events.ObserverList[*voicekit.RegisterWorkerResponse]
	AvailabilityRequests    *events.ObserverList[*voicekit.AvailabilityRequest]
	JobAssignments          *events.ObserverList[*voicekit.JobAssignment]
	JobTerminations         *events.ObserverList[*voicekit.JobTermination]
	WorkerPongs             *events.ObserverList[*voicekit.WorkerPong]
}

func (w *AgentWorker) statusWorker() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()

	for !w.fuse.IsBroken() {
		w.sendStatus()
		<-t.C
	}
}

func (w *AgentWorker) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.fuse.Break()
	return nil
}

func (w *AgentWorker) SetReadDeadline(t time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.fuse.IsBroken() {
		cancel := w.cancel
		if t.IsZero() {
			w.ctx, w.cancel = context.WithCancel(context.Background())
		} else {
			w.ctx, w.cancel = context.WithDeadline(context.Background(), t)
		}
		cancel()
	}
	return nil
}

func (w *AgentWorker) ReadWorkerMessage() (*voicekit.WorkerMessage, int, error) {
	for {
		w.mu.Lock()
		ctx := w.ctx
		w.mu.Unlock()

		select {
		case <-w.fuse.Watch():
			return nil, 0, io.EOF
		case <-ctx.Done():
			if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
				return nil, 0, err
			}
		case m := <-w.workerMessages:
			return m, 0, nil
		}
	}
}

func (w *AgentWorker) WriteServerMessage(m *voicekit.ServerMessage) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.serverMessages.PushBack(m)
	if w.serverMessages.Len() == 1 {
		go w.handleServerMessages()
	}
	return 0, nil
}

func (w *AgentWorker) handleServerMessages() {
	w.mu.Lock()
	for w.serverMessages.Len() != 0 {
		m := w.serverMessages.Front()
		w.mu.Unlock()

		switch m := m.Message.(type) {
		case *voicekit.ServerMessage_Register:
			w.handleRegister(m.Register)
		case *voicekit.ServerMessage_Availability:
			w.handleAvailability(m.Availability)
		case *voicekit.ServerMessage_Assignment:
			w.handleAssignment(m.Assignment)
		case *voicekit.ServerMessage_Termination:
			w.handleTermination(m.Termination)
		case *voicekit.ServerMessage_Pong:
			w.handlePong(m.Pong)
		}

		w.mu.Lock()
		w.serverMessages.PopFront()
	}
	w.mu.Unlock()
}

func (w *AgentWorker) handleRegister(m *voicekit.RegisterWorkerResponse) {
	w.RegisterWorkerResponses.Emit(m)
}

func (w *AgentWorker) handleAvailability(m *voicekit.AvailabilityRequest) {
	w.AvailabilityRequests.Emit(m)
	if w.HandleAvailability != nil {
		w.HandleAvailability(AgentJobRequest{w, m})
	} else {
		AgentJobRequest{w, m}.Accept()
	}
}

func (w *AgentWorker) handleAssignment(m *voicekit.JobAssignment) {
	w.JobAssignments.Emit(m)

	var load JobLoad
	if w.HandleAssignment != nil {
		load = w.HandleAssignment(m.Job)
	}

	if load == nil {
		load = NewStableJobLoad(w.DefaultJobLoad)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.jobs[m.Job.Id] = &AgentJob{m.Job, load}
}

func (w *AgentWorker) handleTermination(m *voicekit.JobTermination) {
	w.JobTerminations.Emit(m)

	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.jobs, m.JobId)
}

func (w *AgentWorker) handlePong(m *voicekit.WorkerPong) {
	w.WorkerPongs.Emit(m)
}

func (w *AgentWorker) sendMessage(m *voicekit.WorkerMessage) {
	select {
	case <-w.fuse.Watch():
	case w.workerMessages <- m:
	}
}

func (w *AgentWorker) SendRegister(m *voicekit.RegisterWorkerRequest) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_Register{
		Register: m,
	}})
}

func (w *AgentWorker) SendAvailability(m *voicekit.AvailabilityResponse) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_Availability{
		Availability: m,
	}})
}

func (w *AgentWorker) SendUpdateWorker(m *voicekit.UpdateWorkerStatus) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_UpdateWorker{
		UpdateWorker: m,
	}})
}

func (w *AgentWorker) SendUpdateJob(m *voicekit.UpdateJobStatus) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_UpdateJob{
		UpdateJob: m,
	}})
}

func (w *AgentWorker) SendPing(m *voicekit.WorkerPing) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_Ping{
		Ping: m,
	}})
}

func (w *AgentWorker) SendSimulateJob(m *voicekit.SimulateJobRequest) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_SimulateJob{
		SimulateJob: m,
	}})
}

func (w *AgentWorker) SendMigrateJob(m *voicekit.MigrateJobRequest) {
	w.sendMessage(&voicekit.WorkerMessage{Message: &voicekit.WorkerMessage_MigrateJob{
		MigrateJob: m,
	}})
}

func (w *AgentWorker) sendStatus() {
	w.mu.Lock()
	var load float32
	jobCount := len(w.jobs)

	if len(w.jobs) == 0 {
		load = w.DefaultWorkerLoad
	} else {
		for _, j := range w.jobs {
			load += j.Load()
		}
	}
	w.mu.Unlock()

	status := voicekit.WorkerStatus_WS_AVAILABLE
	if load > w.JobLoadThreshold {
		status = voicekit.WorkerStatus_WS_FULL
	}

	w.SendUpdateWorker(&voicekit.UpdateWorkerStatus{
		Status:   &status,
		Load:     load,
		JobCount: uint32(jobCount),
	})
}

func (w *AgentWorker) Register(agentName string, jobType voicekit.JobType) {
	w.SendRegister(&voicekit.RegisterWorkerRequest{
		Type:      jobType,
		AgentName: agentName,
	})
	go w.statusWorker()
}

func (w *AgentWorker) SimulateRoomJob(roomName string) {
	w.SendSimulateJob(&voicekit.SimulateJobRequest{
		Type: voicekit.JobType_JT_ROOM,
		Room: &voicekit.Room{
			Sid:  guid.New(guid.RoomPrefix),
			Name: roomName,
		},
	})
}

func (w *AgentWorker) Jobs() []*AgentJob {
	w.mu.Lock()
	defer w.mu.Unlock()
	return maps.Values(w.jobs)
}

type stableJobLoad struct {
	load float32
}

func NewStableJobLoad(load float32) JobLoad {
	return stableJobLoad{load}
}

func (s stableJobLoad) Load() float32 {
	return s.load
}

type periodicJobLoad struct {
	amplitude float64
	period    time.Duration
	epoch     time.Time
}

func NewPeriodicJobLoad(max float32, period time.Duration) JobLoad {
	return periodicJobLoad{
		amplitude: float64(max / 2),
		period:    period,
		epoch:     time.Now().Add(-time.Duration(rand.Int64N(int64(period)))),
	}
}

func (s periodicJobLoad) Load() float32 {
	a := math.Sin(time.Since(s.epoch).Seconds() / s.period.Seconds() * math.Pi * 2)
	return float32(s.amplitude + a*s.amplitude)
}

type uniformRandomJobLoad struct {
	min, max float32
	rng      func() float64
}

func NewUniformRandomJobLoad(min, max float32) JobLoad {
	return uniformRandomJobLoad{min, max, rand.Float64}
}

func NewUniformRandomJobLoadWithRNG(min, max float32, rng *rand.Rand) JobLoad {
	return uniformRandomJobLoad{min, max, rng.Float64}
}

func (s uniformRandomJobLoad) Load() float32 {
	return rand.Float32()*(s.max-s.min) + s.min
}

type normalRandomJobLoad struct {
	mean, stddev float64
	rng          func() float64
}

func NewNormalRandomJobLoad(mean, stddev float64) JobLoad {
	return normalRandomJobLoad{mean, stddev, rand.Float64}
}

func NewNormalRandomJobLoadWithRNG(mean, stddev float64, rng *rand.Rand) JobLoad {
	return normalRandomJobLoad{mean, stddev, rng.Float64}
}

func (s normalRandomJobLoad) Load() float32 {
	u := 1 - s.rng()
	v := s.rng()
	z := math.Sqrt(-2*math.Log(u)) * math.Cos(2*math.Pi*v)
	return float32(max(0, z*s.stddev+s.mean))
}
