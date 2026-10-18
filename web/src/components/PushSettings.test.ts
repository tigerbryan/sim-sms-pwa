import {mount,flushPromises} from '@vue/test-utils'
import {beforeEach,afterEach,describe,expect,it,vi} from 'vitest'
import PushSettings from './PushSettings.vue'
import {api} from '../api/client'
vi.mock('../api/client',()=>({api:vi.fn(),jsonBody:(method:string,value:unknown)=>({method,body:JSON.stringify(value)})}))
let wrapper:ReturnType<typeof mount>
const permission=vi.fn()
const sub={toJSON:()=>({endpoint:'https://web.push.apple.com/test',keys:{p256dh:'test',auth:'test'}}),unsubscribe:vi.fn().mockResolvedValue(true)}
const subscribe=vi.fn().mockResolvedValue(sub)
beforeEach(()=>{
 vi.clearAllMocks()
 vi.stubGlobal('Notification',{permission:'default',requestPermission:permission.mockResolvedValue('granted')})
 vi.stubGlobal('PushManager',class {})
 Object.defineProperty(window,'isSecureContext',{configurable:true,value:true})
 Object.defineProperty(navigator,'userAgent',{configurable:true,value:'Chrome'})
 Object.defineProperty(navigator,'serviceWorker',{configurable:true,value:{ready:Promise.resolve({pushManager:{getSubscription:vi.fn().mockResolvedValue(null),subscribe}})}})
 window.matchMedia=vi.fn().mockReturnValue({matches:false})
 vi.mocked(api).mockImplementation(async path=> path.includes('/status')?{public_key:'AQID'}:path.endsWith('/subscribe')?{id:'device1'}:{queued:true})
})
afterEach(()=>{wrapper?.unmount();vi.unstubAllGlobals()})
describe('Native push setup',()=>{
 it('asks for system permission from the tap and persists the actual browser subscription',async()=>{
  Object.defineProperty(navigator,'userAgent',{configurable:true,value:'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/152.0.0.0'})
  wrapper=mount(PushSettings);await flushPromises()
  expect(wrapper.find('section').attributes('aria-label')).toBe('此设备系统通知')
  expect(wrapper.find('button').text()).toBe('启用系统通知')
  expect(wrapper.text()).toContain('系统设置 → 通知')
  expect(wrapper.text()).toContain('标记应用程序图标')
  await wrapper.find('button').trigger('click');await flushPromises()
  expect(permission).toHaveBeenCalledOnce()
  expect(subscribe).toHaveBeenCalledWith(expect.objectContaining({userVisibleOnly:true}))
  expect(api).toHaveBeenCalledWith('/api/v1/webpush/subscribe',expect.objectContaining({method:'POST',body:JSON.stringify(sub.toJSON())}))
  expect(wrapper.text()).toContain('已启用')
 })
 it('requires opening the iPhone PWA from the home screen before requesting permission',async()=>{
  Object.defineProperty(navigator,'userAgent',{configurable:true,value:'iPhone'})
  wrapper=mount(PushSettings);await flushPromises()
  expect(wrapper.find('button').attributes('disabled')).toBeDefined()
  expect(wrapper.text()).toContain('添加到主屏幕')
  expect(permission).not.toHaveBeenCalled()
 })
 it('does not report success or store a subscription when permission is denied',async()=>{
  permission.mockResolvedValue('denied');wrapper=mount(PushSettings);await flushPromises()
  await wrapper.find('button').trigger('click');await flushPromises()
  expect(wrapper.find('[role=alert]').text()).toContain('通知已被拒绝')
  expect(subscribe).not.toHaveBeenCalled()
 })
})
