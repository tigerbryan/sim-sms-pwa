import {describe,expect,it,vi} from 'vitest'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
const source=readFileSync('public/sw.js','utf8')
function worker(navigator: Record<string,unknown> = {}){
 const handlers:Record<string,(event:any)=>void>={}
 const showNotification=vi.fn().mockResolvedValue(undefined)
 const self={addEventListener:(name:string,fn:(event:any)=>void)=>handlers[name]=fn,location:{origin:'https://sms.example.test'},skipWaiting:vi.fn().mockResolvedValue(undefined),registration:{showNotification},clients:{matchAll:vi.fn().mockResolvedValue([]),openWindow:vi.fn().mockResolvedValue(null)}}
 const fetch=vi.fn()
 vm.runInNewContext(source,{self,navigator,URL,fetch,caches:{}})
 return {self,handlers,showNotification,fetch}
}
describe('Lock-screen push worker',()=>{
 it('activates a downloaded update only after an explicit application message',async()=>{
  const w=worker();let completed!:Promise<unknown>
  expect(w.self.skipWaiting).not.toHaveBeenCalled()
  w.handlers.message!({data:{type:'SKIP_WAITING'},waitUntil:(p:Promise<unknown>)=>completed=p})
  await completed;expect(w.self.skipWaiting).toHaveBeenCalledOnce()
 })
 it('shows a notification without fetching protected SMS or needing an open window',async()=>{
  const w=worker();let completed!:Promise<unknown>
  w.handlers.push!({data:{json:()=>({title:'信息',body:'收到新短信',tag:'sms:1',unread:2})},waitUntil:(p:Promise<unknown>)=>completed=p})
  await completed
  expect(w.showNotification).toHaveBeenCalledWith('信息',expect.objectContaining({body:'收到新短信',tag:'sms:1',data:{url:'/sms'}}))
  expect(w.fetch).not.toHaveBeenCalled()
 })
 it('still shows a visible fallback when push data cannot be decoded',async()=>{
  const w=worker();let completed!:Promise<unknown>
  w.handlers.push!({data:{json:()=>{throw new Error('invalid')}},waitUntil:(p:Promise<unknown>)=>completed=p})
  await completed;expect(w.showNotification).toHaveBeenCalledOnce()
 })
 it('opens the protected inbox and ignores arbitrary payload navigation URLs',async()=>{
  const w=worker();let completed!:Promise<unknown>;const close=vi.fn()
  w.handlers.notificationclick!({notification:{close,data:{url:'https://untrusted.test'}},waitUntil:(p:Promise<unknown>)=>completed=p})
  await completed;expect(close).toHaveBeenCalledOnce();expect(w.self.clients.openWindow).toHaveBeenCalledWith('https://sms.example.test/sms')
 })
 it.each([null, [], 'invalid', {unread:-1}, {unread:'3'}, {unread:1.5}])('preserves unread badges for invalid or missing counts: %j',async payload=>{
  const setAppBadge=vi.fn(),clearAppBadge=vi.fn(),w=worker({setAppBadge,clearAppBadge});let completed!:Promise<unknown>
  w.handlers.push!({data:{json:()=>payload},waitUntil:(p:Promise<unknown>)=>completed=p})
  await completed
  expect(w.showNotification).toHaveBeenCalledOnce()
  expect(setAppBadge).not.toHaveBeenCalled();expect(clearAppBadge).not.toHaveBeenCalled()
 })
 it('sets the reported unread badge and clears it only for an explicit zero',async()=>{
  const setAppBadge=vi.fn(),clearAppBadge=vi.fn(),w=worker({setAppBadge,clearAppBadge});let completed!:Promise<unknown>
  w.handlers.push!({data:{json:()=>({unread:2})},waitUntil:(p:Promise<unknown>)=>completed=p});await completed
  expect(setAppBadge).toHaveBeenCalledWith(2);expect(clearAppBadge).not.toHaveBeenCalled()
  w.handlers.push!({data:{json:()=>({unread:0})},waitUntil:(p:Promise<unknown>)=>completed=p});await completed
  expect(clearAppBadge).toHaveBeenCalledOnce()
 })

})
