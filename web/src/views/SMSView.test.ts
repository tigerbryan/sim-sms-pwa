import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import SMSView from './SMSView.vue'
import { getSMSPage, setSMSRead } from '../api/sms'
import type { SMSMessage } from '../types/api'
vi.mock('../api/sms', () => ({ getSMSPage: vi.fn(), setSMSRead: vi.fn() }))
const sms = (id: number, sender = '10010', body = '测试短信'): SMSMessage => ({ id, sender, body, device_id:'ec25', timestamp:'2026-10-10T06:00:00Z', received_at:`2026-10-10T06:0${id}:00Z`, encoding:'GSM7', is_multipart:false, parts_total:1, parts_received:1, status:'unread' })
const response = (items: SMSMessage[]) => ({items, total:items.length, page:1, page_size:100, per_page:100})
let wrapper: ReturnType<typeof mount>
let serviceWorker: EventTarget
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return {promise, resolve}
}
beforeEach(() => {
  vi.useFakeTimers(); vi.resetAllMocks()
  serviceWorker = new EventTarget()
  Object.defineProperty(navigator, 'serviceWorker', {configurable:true, value:serviceWorker})
  Object.defineProperty(document, 'hidden', {configurable:true, value:false})
  vi.mocked(setSMSRead).mockResolvedValue({updated:true})
})
afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })
const render = () => mount(SMSView, {global:{stubs:{RouterLink:true}}})
describe('Messages inbox', () => {
  it('groups a sender into one conversation and marks all opened messages read', async () => {
    vi.mocked(getSMSPage).mockResolvedValue(response([sms(1), sms(2), sms(3,'10086')]))
    wrapper=render(); await flushPromises()
    expect(wrapper.findAll('.thread')).toHaveLength(2)
    await wrapper.findAll('.thread')[1]!.trigger('click'); await flushPromises()
    expect(wrapper.findAll('.bubble')).toHaveLength(2)
    expect(setSMSRead).toHaveBeenCalledWith(1,true)
    expect(setSMSRead).toHaveBeenCalledWith(2,true)
    expect(wrapper.find('textarea').exists()).toBe(false)
  })
  it('automatically merges incoming messages without duplicating an existing conversation', async () => {
    vi.mocked(getSMSPage).mockResolvedValueOnce(response([sms(1)])).mockResolvedValue(response([sms(2,'10010','新短信'),sms(1)]))
    wrapper=render(); await flushPromises()
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(wrapper.findAll('.thread')).toHaveLength(1)
    expect(wrapper.find('.preview').text()).toBe('新短信')
    Object.defineProperty(document,'hidden',{configurable:true,value:true})
    await vi.advanceTimersByTimeAsync(5000)
    expect(getSMSPage).toHaveBeenCalledTimes(2)
  })
  it('searches the stored inbox through the API and continues polling after search', async () => {
    vi.mocked(getSMSPage).mockResolvedValue(response([sms(1)]))
    wrapper=render(); await flushPromises()
    await wrapper.find('input[type="search"]').setValue('验证码')
    await vi.advanceTimersByTimeAsync(300); await flushPromises()
    expect(getSMSPage).toHaveBeenLastCalledWith(1,100,'验证码')
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(getSMSPage).toHaveBeenCalledTimes(3)
  })
  it('immediately follows a push received while a refresh is still in flight', async () => {
    const inFlight = deferred<ReturnType<typeof response>>()
    vi.mocked(getSMSPage).mockResolvedValueOnce(response([sms(1)]))
      .mockReturnValueOnce(inFlight.promise)
      .mockResolvedValue(response([sms(2, '10010', '推送后的短信'), sms(1)]))
    wrapper=render(); await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    serviceWorker.dispatchEvent(new MessageEvent('message', {data:{type:'sms-push'}}))
    serviceWorker.dispatchEvent(new MessageEvent('message', {data:{type:'sms-push'}}))
    expect(getSMSPage).toHaveBeenCalledTimes(2)
    inFlight.resolve(response([sms(1)])); await flushPromises()
    expect(getSMSPage).toHaveBeenCalledTimes(3)
    expect(wrapper.find('.preview').text()).toBe('推送后的短信')
  })
  it('marks the newly opened conversation read after a slow request for the previous conversation', async () => {
    const firstRead = deferred<{updated:boolean}>()
    vi.mocked(getSMSPage).mockResolvedValue(response([sms(1, '10010'), sms(2, '10086')]))
    vi.mocked(setSMSRead).mockReturnValueOnce(firstRead.promise).mockResolvedValue({updated:true})
    wrapper=render(); await flushPromises()
    await wrapper.findAll('.thread')[0]!.trigger('click'); await flushPromises()
    await wrapper.findAll('.thread')[1]!.trigger('click'); await flushPromises()
    expect(setSMSRead).toHaveBeenCalledTimes(1)
    firstRead.resolve({updated:true}); await flushPromises()
    expect(setSMSRead).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('.unread-dot.visible')).toHaveLength(0)
  })
  it('does not start another read request after the app is hidden', async () => {
    const firstRead = deferred<{updated:boolean}>()
    vi.mocked(getSMSPage).mockResolvedValue(response([sms(1), sms(2)]))
    vi.mocked(setSMSRead).mockReturnValueOnce(firstRead.promise)
    wrapper=render(); await flushPromises()
    await wrapper.find('.thread').trigger('click'); await flushPromises()
    Object.defineProperty(document, 'hidden', {configurable:true, value:true})
    firstRead.resolve({updated:true}); await flushPromises()
    expect(setSMSRead).toHaveBeenCalledTimes(1)
  })

})
