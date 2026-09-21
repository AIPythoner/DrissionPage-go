package drissionpage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// DisplayRecording uses the browser's screen-sharing picker and MediaRecorder.
// Start requires a secure page and the user's screen-sharing permission. The
// selected surface may be a tab, window or display, as in Python js_video mode.
type DisplayRecording struct {
	tab   *ChromiumTab
	key   string
	mu    sync.Mutex
	saved bool
}

func (t *ChromiumTab) StartDisplayRecording(ctx context.Context) (*DisplayRecording, error) {
	keyData, err := t.RunJS(ctx, `async()=>{
 const key='__dp_record_'+crypto.randomUUID();
 const stream=await navigator.mediaDevices.getDisplayMedia({video:true,audio:false,selfBrowserSurface:"include"});
 try {
 const recorder=new MediaRecorder(stream,{mimeType:'video/webm'});
 const state={recorder,stream,chunks:[],error:null};
 state.done=new Promise(resolve=>recorder.addEventListener('stop',resolve,{once:true}));
 recorder.addEventListener('dataavailable',e=>{if(e.data.size)state.chunks.push(e.data)});
 recorder.addEventListener('error',e=>state.error=String(e.error));
 globalThis[key]=state;recorder.start(200);return key;
 } catch(e){stream.getTracks().forEach(t=>t.stop());throw e}
 }`)
	if err != nil {
		return nil, err
	}
	var key string
	if err = json.Unmarshal(keyData, &key); err != nil {
		return nil, err
	}
	return &DisplayRecording{tab: t, key: key}, nil
}

// Stop saves WebM without overwriting an existing file. If writing fails, call
// Stop again to retrieve the retained browser-side recording.
func (s *DisplayRecording) Stop(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saved {
		return fmt.Errorf("display recording already saved")
	}
	result, err := s.tab.RunJS(ctx, `async key=>{
 const s=globalThis[key];if(!s)throw Error('recording lost after page navigation');
 if(s.recorder.state!=='inactive')s.recorder.stop();await s.done;s.stream.getTracks().forEach(t=>t.stop());
 if(s.error)throw Error(s.error);
 return await new Promise((resolve,reject)=>{const reader=new FileReader();reader.onerror=()=>reject(reader.error);reader.onload=()=>resolve(reader.result.split(',')[1]);reader.readAsDataURL(new Blob(s.chunks,{type:'video/webm'}))});
 }`, s.key)
	if err != nil {
		return err
	}
	var encoded string
	if err = json.Unmarshal(result, &encoded); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("display recording is empty")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		os.Remove(path)
		return err
	}
	if closeErr != nil {
		os.Remove(path)
		return closeErr
	}
	s.saved = true
	_, err = s.tab.RunJS(ctx, `key=>{delete globalThis[key]}`, s.key)
	return err
}

func (s *DisplayRecording) Cancel(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.tab.RunJS(ctx, `key=>{const s=globalThis[key];if(s){if(s.recorder.state!=='inactive')s.recorder.stop();s.stream.getTracks().forEach(t=>t.stop());delete globalThis[key]}}`, s.key)
	return err
}
