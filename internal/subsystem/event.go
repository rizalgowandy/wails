package subsystem

import (
	"strings"
	"sync"

	"github.com/leaanthony/wailsv2/v2/internal/logger"
	"github.com/leaanthony/wailsv2/v2/internal/messagedispatcher/message"
	"github.com/leaanthony/wailsv2/v2/internal/servicebus"
)

// eventListener holds a callback function which is invoked when
// the event listened for is emitted. It has a counter which indicates
// how the total number of events it is interested in. A value of zero
// means it does not expire (default).
type eventListener struct {
	callback func(...interface{}) // Function to call with emitted event data
	counter  int64                // The number of times this callback may be called. -1 = infinite
	delete   bool                 // Flag to indicate that this listener should be deleted
}

// Event is the Eventing subsystem. It manages all service bus messages
// starting with "event".
type Event struct {
	quitChannel  <-chan *servicebus.Message
	eventChannel <-chan *servicebus.Message
	running      bool

	// Event listeners
	listeners  map[string][]*eventListener
	notifyLock sync.RWMutex

	// logger
	logger logger.CustomLogger
}

// NewEvent creates a new log subsystem
func NewEvent(bus *servicebus.ServiceBus, logger *logger.Logger) (*Event, error) {

	// Register quit channel
	quitChannel, err := bus.Subscribe("quit")
	if err != nil {
		return nil, err
	}

	// Subscribe to event messages
	eventChannel, err := bus.Subscribe("event")
	if err != nil {
		return nil, err
	}

	result := &Event{
		quitChannel:  quitChannel,
		eventChannel: eventChannel,
		logger:       logger.CustomLogger("Event Subsystem"),
		listeners:    make(map[string][]*eventListener),
	}

	return result, nil
}

// RegisterListener provides a means of subscribing to events of type "eventName"
func (e *Event) RegisterListener(eventName string, callback func(...interface{})) {

	// Create new eventListener
	thisListener := &eventListener{
		callback: callback,
		counter:  0,
		delete:   false,
	}

	e.notifyLock.Lock()
	// Append the new listener to the listeners slice
	e.listeners[eventName] = append(e.listeners[eventName], thisListener)
	e.notifyLock.Unlock()
}

// Start the subsystem
func (e *Event) Start() error {

	e.logger.Trace("Starting")

	e.running = true

	// Spin off a go routine
	go func() {
		for e.running {
			select {
			case <-e.quitChannel:
				e.running = false
				break
			case eventMessage := <-e.eventChannel:
				splitTopic := strings.Split(eventMessage.Topic(), ":")
				eventType := splitTopic[1]
				switch eventType {
				case "emit":
					if len(splitTopic) != 4 {
						e.logger.Error("Received emit message with invalid topic format. Expected 4 sections in topic, got %s", splitTopic)
						continue
					}
					eventSource := splitTopic[3]
					e.logger.Trace("Got Event Message: %s %+v", eventMessage.Topic(), eventMessage.Data())
					event := eventMessage.Data().(*message.EventMessage)
					eventName := event.Name
					switch eventSource {

					case "j":
						// Notify Go Subscribers
						e.logger.Trace("Notify Go subscribers to event '%s'", eventName)
						go e.notifyListeners(eventName, event)
					case "g":
						// Notify Go listeners
						e.logger.Trace("Got Go Event: %s", eventName)
						go e.notifyListeners(eventName, event)
					default:
						e.logger.Error("unknown emit event message: %+v", eventMessage)
					}
				case "on":
					// We wish to subscribe to an event channel
					var message *message.OnEventMessage = eventMessage.Data().(*message.OnEventMessage)
					eventName := message.Name
					callback := message.Callback
					e.RegisterListener(eventName, callback)
					e.logger.Trace("Registered listener for event '%s' with callback %p", eventName, callback)
				default:
					e.logger.Error("unknown event message: %+v", eventMessage)
				}
			}
		}

		// Call shutdown
		e.shutdown()
	}()

	return nil
}

// Notifies listeners for the given event name
func (e *Event) notifyListeners(eventName string, message *message.EventMessage) {

	// Get list of event listeners
	listeners := e.listeners[eventName]
	if listeners == nil {
		println("no listeners for", eventName)
		return
	}

	// Lock the listeners
	e.notifyLock.Lock()

	// We have a dirty flag to indicate that there are items to delete
	itemsToDelete := false

	// Callback in goroutine
	for _, listener := range listeners {
		if listener.counter > 0 {
			listener.counter--
		}

		go listener.callback(message.Data...)

		if listener.counter == 0 {
			listener.delete = true
			itemsToDelete = true
		}
	}

	// Do we have items to delete?
	if itemsToDelete == true {

		// Create a new Listeners slice
		var newListeners = []*eventListener{}

		// Iterate over current listeners
		for _, listener := range listeners {
			// If we aren't deleting the listener, add it to the new list
			if !listener.delete {
				newListeners = append(newListeners, listener)
			}
		}

		// Save new listeners
		e.listeners[eventName] = newListeners
	}

	// Unlock
	e.notifyLock.Unlock()

}

func (e *Event) shutdown() {
	e.logger.Trace("Shutdown")
}
