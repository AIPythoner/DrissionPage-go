# Python 公开成员目录

基准：DrissionPage 5.0.0b1 / b46345b。由 AST 提取类中直接定义的公开方法/属性，不重复展开继承成员。此目录用于检查迁移范围，不是逐成员测试通过率。Go 对应见 [迁移清单](MIGRATION.md)。

## _base/base.py

### BaseParser

`ele`, `eles`, `find`, `html`, `s_ele`, `s_eles`

### BaseElement

`get_frame`, `timeout`, `child_count`, `tag`, `parent`, `next`, `nexts`

### DrissionElement

`link`, `css_selector`, `xpath`, `comments`, `texts`, `parent`, `child`, `prev`, `next`, `before`, `after`, `children`, `prevs`, `nexts`, `befores`, `afters`, `attrs`, `text`, `raw_text`, `attr`

### BasePage

`title`, `url_available`, `download_path`, `download`, `url`, `json`, `user_agent`, `get`

## _base/driver.py

### ThreadSafeDict

`get`, `pop`, `clear`

### Driver

`add_session_owner`, `remove_session_owner`, `run`, `start`, `stop`, `get`

### DebugDriver

`run`

## _browsers/chromium.py

### Chromium

`context`, `id`, `user_data_path`, `process_id`, `timeout`, `timeouts`, `load_mode`, `download_path`, `set`, `listen`, `states`, `wait`, `tab_ids`, `latest_tab`, `command_line`, `cookies`, `new_context`, `new_tab`, `get_tab`, `get_tabs`, `close_tabs`, `activate_tab`, `reconnect`, `clear_cache`, `quit`

### Tabs

`session_ids`, `target_ids`, `objects`, `frame_ids`, `openers`, `add`, `add_obj`, `add_frame`, `remove_frame`, `remove_session`, `remove_target`, `remove_context`, `set_proxy`, `get_proxy`, `set_newest_tab`, `get_newest_tab`, `get_session_ids`, `get_target_id`, `get_context_id`, `get_object`, `stop_session`, `stop_target`, `clear`

## _browsers/chromium_context.py

### ChromiumContext

`browser`, `tab_ids`, `latest_tab`, `set`, `wait`, `cookies`, `new_tab`, `get_tab`, `get_tabs`, `close`

## _configs/chromium_options.py

### ChromiumOptions

`download_path`, `browser_path`, `user_data_path`, `tmp_path`, `user`, `load_mode`, `timeouts`, `proxy`, `address`, `arguments`, `extensions`, `preferences`, `flags`, `system_user_path`, `is_existing_only`, `is_auto_port`, `retry_times`, `retry_interval`, `is_headless`, `set_retry`, `set_argument`, `remove_argument`, `add_extension`, `remove_extensions`, `set_pref`, `remove_pref`, `remove_pref_from_file`, `set_flag`, `clear_flags_in_file`, `clear_flags`, `clear_arguments`, `clear_prefs`, `set_timeouts`, `set_user`, `headless`, `no_imgs`, `no_js`, `mute`, `incognito`, `new_env`, `ignore_certificate_errors`, `disable_pdf_preview`, `set_user_agent`, `set_proxy`, `set_load_mode`, `set_local_port`, `set_address`, `set_browser_path`, `use_edge`, `set_download_path`, `set_tmp_path`, `set_user_data_path`, `set_cache_path`, `use_system_user_path`, `auto_port`, `existing_only`, `save`, `save_to_default`, `close_cross_origin`

## _configs/options_manage.py

### OptionsManager

`get_value`, `get_option`, `set_item`, `remove_item`, `save`, `save_to_default`, `show`

## _configs/session_options.py

### SessionOptions

`download_path`, `set_download_path`, `timeout`, `set_timeout`, `proxies`, `set_proxies`, `retry_times`, `retry_interval`, `set_retry`, `headers`, `set_headers`, `set_a_header`, `remove_a_header`, `clear_headers`, `cookies`, `set_cookies`, `auth`, `set_auth`, `hooks`, `set_hooks`, `params`, `set_params`, `verify`, `set_verify`, `cert`, `set_cert`, `adapters`, `add_adapter`, `stream`, `set_stream`, `trust_env`, `set_trust_env`, `max_redirects`, `set_max_redirects`, `save`, `save_to_default`, `as_dict`, `make_session`, `from_session`

## _elements/chromium_element.py

### ChromiumElement

`tag`, `html`, `inner_html`, `attrs`, `text`, `raw_text`, `set`, `states`, `pseudo`, `rect`, `sr`, `shadow_root`, `scroll`, `click`, `wait`, `select`, `value`, `check`, `parent`, `child`, `prev`, `next`, `before`, `after`, `children`, `prevs`, `nexts`, `befores`, `afters`, `over`, `offset`, `east`, `south`, `west`, `north`, `attr`, `remove_attr`, `property`, `run_js`, `run_async_js`, `ele`, `eles`, `s_ele`, `s_eles`, `style`, `src`, `save`, `get_screenshot`, `input`, `clear`, `focus`, `hover`, `drag`, `drag_to`

### ShadowRoot

`tag`, `html`, `inner_html`, `states`, `run_js`, `run_async_js`, `parent`, `child`, `next`, `before`, `after`, `children`, `nexts`, `befores`, `afters`, `ele`, `eles`, `s_ele`, `s_eles`

### Pseudo

`before`, `after`

## _elements/session_element.py

### SessionElement

`inner_ele`, `tag`, `html`, `inner_html`, `attrs`, `text`, `raw_text`, `parent`, `child`, `prev`, `next`, `before`, `after`, `children`, `prevs`, `nexts`, `befores`, `afters`, `attr`, `ele`, `eles`, `s_ele`, `s_eles`

## _functions/cookies.py

### CookiesList

`as_dict`, `as_str`, `as_json`

## _functions/elements.py

### SessionElementsList

`get`, `filter`, `filter_one`, `texts`

### ChromiumElementsList

`filter`, `filter_one`, `search`, `search_one`

### SessionFilterOne

`tag`, `attr`, `text`

### SessionFilter

`get`, `tag`, `text`

### ChromiumFilterOne

`displayed`, `checked`, `selected`, `enabled`, `clickable`, `have_rect`, `style`, `property`

### ChromiumFilter

`get`, `search_one`, `search`, `tag`, `text`

### Getter

`links`, `texts`, `attrs`

## _functions/settings.py

### Settings

`set_wait_stop_before_click`, `set_raise_when_ele_not_found`, `set_raise_when_click_failed`, `set_raise_when_wait_failed`, `set_singleton_tab_obj`, `set_cdp_timeout`, `set_browser_connect_timeout`, `set_auto_handle_alert`, `set_language`, `set_suffixes_list`

## _functions/texts.py

### Texts

`get`, `join`, `joinn`

## _functions/tools.py

### PortFinder

`get_port`

## _functions/web.py

### NavResult

`ok`

## _pages/chromium_base.py

### ChromiumBase

`wait`, `set`, `screencast`, `actions`, `listen`, `states`, `scroll`, `rect`, `console`, `timeout`, `timeouts`, `browser`, `driver`, `title`, `url`, `html`, `json`, `tab_id`, `active_ele`, `load_mode`, `user_agent`, `upload_list`, `session`, `run_cdp`, `run_cdp_loaded`, `run_js`, `run_js_loaded`, `run_async_js`, `get`, `cookies`, `ele`, `eles`, `s_ele`, `s_eles`, `refresh`, `forward`, `back`, `stop_loading`, `remove_ele`, `new_ele`, `get_frame`, `get_frames`, `session_storage`, `local_storage`, `get_screenshot`, `add_init_js`, `remove_init_js`, `clear_cache`, `disconnect`, `handle_alert`

### Timeout

`as_dict`

## _pages/chromium_frame.py

### ChromiumFrame

`scroll`, `set`, `states`, `wait`, `rect`, `listen`, `owner`, `frame_ele`, `tag`, `url`, `html`, `inner_html`, `link`, `title`, `attrs`, `active_ele`, `xpath`, `css_selector`, `tab`, `tab_id`, `download_path`, `sr`, `shadow_root`, `child_count`, `refresh`, `property`, `attr`, `remove_attr`, `style`, `run_js`, `parent`, `prev`, `next`, `before`, `after`, `prevs`, `nexts`, `befores`, `afters`, `get_screenshot`

## _pages/chromium_tab.py

### ChromiumTab

`set`, `wait`, `url`, `title`, `raw_data`, `html`, `json`, `response`, `mode`, `user_agent`, `timeout`, `activate`, `get`, `post`, `ele`, `eles`, `s_ele`, `s_eles`, `change_mode`, `cookies_to_session`, `cookies_to_browser`, `cookies`, `close`, `save`

## _pages/session_page.py

### SessionPage

`title`, `url`, `raw_data`, `html`, `json`, `user_agent`, `session`, `response`, `encoding`, `set`, `timeout`, `get`, `post`, `ele`, `eles`, `s_ele`, `s_eles`, `cookies`, `close`

## _units/actions.py

### Actions

`move_to`, `move`, `click`, `r_click`, `m_click`, `hold`, `release`, `r_hold`, `r_release`, `m_hold`, `m_release`, `scroll`, `up`, `down`, `left`, `right`, `key_down`, `key_up`, `type`, `input`, `drag_in`, `wait`

## _units/clicker.py

### Clicker

`left`, `right`, `middle`, `at`, `multi`, `to_download`, `to_upload`, `for_new_tab`, `for_url_change`, `for_title_change`

## _units/console.py

### Console

`messages`, `start`, `stop`, `clear`, `wait`, `steps`

### ConsoleData

`data`

## _units/cookies_setter.py

### BrowserCookiesSetter

`clear`

### CookiesSetter

`remove`, `clear`

### ChromiumTabCookiesSetter

`remove`, `clear`

### SessionCookiesSetter

`remove`, `clear`

## _units/downloader.py

### DownloadManager

`missions`, `set_path`, `set_rename`, `set_file_exists`, `set_flag`, `get_flag`, `get_tab_missions`, `set_done`, `cancel`, `skip`, `clear_tab_info`

### DownloadMission

`rate`, `is_done`, `cancel`, `wait`

## _units/listener.py

### BaseListener

`urls`, `set_method`, `set_res_type`, `set_urls`, `resume`

### Listener

`start`, `wait`, `browser_wait`, `steps`, `browser_steps`, `stop`, `pause`, `clear`, `wait_silent`

### BrowserListener

`start`, `stop`, `pause`, `clear`

### DataPacket

`url`, `method`, `frameId`, `resourceType`, `request`, `response`, `data`, `fail_info`, `wait_extra_info`

### Request

`headers`, `params`, `postData`, `cookies`, `extra_info`, `timestamp`

### Response

`headers`, `raw_body`, `body`, `extra_info`, `timestamp`

### ExtraInfo

`all_info`

### WebSocketPacket

`timestamp`, `data`, `url`, `method`, `frameId`, `request`, `response`, `is_failed`

### SSEPacket

`timestamp`, `name`, `id`, `data`, `url`, `method`, `frameId`, `request`, `response`, `is_failed`

### MethodSetter

`all`

### ResTypeSetter

`ws`, `remove_ws`, `sse`, `remove_sse`

### BrowserDataPacket

`url`, `method`, `frameId`, `request`, `response`, `data`, `resourceType`, `is_failed`, `timestamp`

### BrowserRequest

`headers`, `params`, `postData`

### BrowserResponse

`body`

## _units/perm_setter.py

### BrowserPermSetter

`geolocation`, `notifications`, `push`, `midi`, `camera`, `microphone`, `background_fetch`, `background_sync`, `persistent_storage`, `ambient_light_sensor`, `accelerometer`, `gyroscope`, `magnetometer`, `screen_wake_lock`, `nfc`, `display_capture`, `clipboard_read`, `clipboard_write`, `payment_handler`, `idle_detection`, `periodic_background_sync`, `system_wake_lock`, `storage_access`, `window_management`, `local_fonts`, `top_level_storage_access`, `captured_surface_control`, `speaker_selection`, `keyboard_lock`, `pointer_lock`, `fullscreen`, `web_app_installation`, `local_network_access`, `local_network`, `loopback_network`

## _units/rect.py

### ElementRect

`corners`, `viewport_corners`, `size`, `location`, `midpoint`, `click_point`, `viewport_location`, `viewport_midpoint`, `viewport_click_point`, `screen_location`, `screen_midpoint`, `screen_click_point`, `scroll_position`

### TabRect

`window_state`, `window_location`, `window_size`, `page_location`, `viewport_location`, `size`, `viewport_size`, `viewport_size_with_scrollbar`, `scroll_position`

### FrameRect

`location`, `viewport_location`, `screen_location`, `size`, `viewport_size`, `corners`, `viewport_corners`, `scroll_position`

## _units/screencast.py

### Screencast

`set_mode`, `start`, `stop`, `set_save_path`

### ScreencastMode

`video_mode`, `frugal_video_mode`, `js_video_mode`, `frugal_imgs_mode`, `imgs_mode`

## _units/scroller.py

### Scroller

`to_top`, `to_bottom`, `to_half`, `to_rightmost`, `to_leftmost`, `to_location`, `up`, `down`, `left`, `right`

### ElementScroller

`to_see`, `to_center`

### PageScroller

`to_see`

### FrameScroller

`to_see`

## _units/selector.py

### SelectElement

`is_multi`, `options`, `selected_option`, `selected_options`, `all`, `invert`, `clear`, `by_text`, `by_value`, `by_index`, `by_locator`, `by_option`, `cancel_by_text`, `cancel_by_value`, `cancel_by_index`, `cancel_by_locator`, `cancel_by_option`

## _units/setter.py

### BaseSetter

`NoneElement_value`, `retry_times`, `retry_interval`, `download_path`

### SessionPageSetter

`cookies`, `download_path`, `timeout`, `encoding`, `headers`, `header`, `user_agent`, `proxies`, `auth`, `hooks`, `params`, `verify`, `cert`, `stream`, `trust_env`, `max_redirects`, `add_adapter`

### BrowserContextSetter

`perm`, `cookies`

### BrowserBaseSetter

`load_mode`, `timeouts`

### BrowserSetter

`auto_handle_alert`, `download_path`, `download_file_name`, `when_download_file_exists`

### ChromiumBaseSetter

`scroll`, `cookies`, `headers`, `user_agent`, `session_storage`, `local_storage`, `upload_files`, `auto_handle_alert`, `blocked_urls`, `show_trail`

### ChromiumTabSetter

`window`, `cookies`, `headers`, `user_agent`, `timeouts`, `download_path`, `download_file_name`, `when_download_file_exists`

### ChromiumElementSetter

`attr`, `property`, `style`, `innerHTML`, `value`

### ChromiumFrameSetter

`attr`, `property`, `style`

### LoadMode

`normal`, `eager`, `none`

### PageScrollSetter

`wait_complete`, `smooth`

### WindowSetter

`max`, `mini`, `full`, `normal`, `size`, `location`, `hide`, `show`

## _units/states.py

### ElementStates

`is_selected`, `is_checked`, `is_displayed`, `is_enabled`, `is_alive`, `is_in_viewport`, `is_whole_in_viewport`, `is_covered`, `is_clickable`, `has_rect`

### ShadowRootStates

`is_enabled`, `is_alive`

### BrowserStates

`is_alive`, `is_headless`, `is_existed`, `is_incognito`, `is_guest`

### PageStates

`is_loading`, `is_alive`, `ready_state`, `has_alert`, `is_headless`, `is_existed`, `is_incognito`

### FrameStates

`is_loading`, `is_alive`, `ready_state`, `is_displayed`, `has_alert`

## _units/waiter.py

### BrowserContextWaiter

`new_tab`

### BrowserWaiter

`download_begin`, `downloads_done`

### BaseWaiter

`ele_deleted`, `ele_displayed`, `ele_hidden`, `eles_loaded`, `load_start`, `doc_loaded`, `upload_paths_inputted`, `download_begin`, `url_change`, `title_change`

### ChromiumTabWaiter

`downloads_done`, `alert_closed`

### ElementWaiter

`deleted`, `displayed`, `hidden`, `covered`, `not_covered`, `enabled`, `disabled`, `disabled_or_deleted`, `clickable`, `has_rect`, `stop_moving`

共 858 个直接定义的公开成员，包括内部模块中的公开命名类成员。此数不是 Go 实现数量或兼容性百分比。
