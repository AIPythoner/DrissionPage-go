package drissionpage

func (l *Listener) PacketStats() QueueStats        { return l.packets.stats() }
func (l *Listener) StreamStats() QueueStats        { return l.streams.stats() }
func (l *BrowserListener) PacketStats() QueueStats { return l.packets.stats() }
func (l *BrowserListener) StreamStats() QueueStats { return l.streams.stats() }
func (l *BrowserListener) ErrorStats() QueueStats  { return l.errs.stats() }
func (c *Console) QueueStats() QueueStats          { return c.queue.stats() }
func (c *Console) SetBufferSize(limit int)         { c.queue.setLimit(limit) }
func (d *DownloadManager) QueueStats() QueueStats  { return d.queue.stats() }
