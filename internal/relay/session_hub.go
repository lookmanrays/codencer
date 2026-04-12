package relay

type InstanceStatus struct {
	Online bool   `json:"online"`
	Status string `json:"status"`
}

func (s *Server) instanceStatus(record *InstanceRecord) InstanceStatus {
	if record == nil {
		return InstanceStatus{Online: false, Status: "not_found"}
	}
	if s.hub.Get(record.InstanceID) != nil {
		return InstanceStatus{Online: true, Status: "online"}
	}
	return InstanceStatus{Online: false, Status: "offline"}
}
