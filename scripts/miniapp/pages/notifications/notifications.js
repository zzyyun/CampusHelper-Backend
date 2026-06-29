const api=require('../../utils/api')
const {NOTIFY_TYPE_TEXT}=require('../../utils/constants')
Page({
  data:{notifications:[],unreadCount:0,cursor:'',hasMore:true,loading:false,notifyTypeText:NOTIFY_TYPE_TEXT},
  onShow(){this.loadUnreadCount();if(this.data.notifications.length===0)this.loadNotifications()},
  onPullDownRefresh(){this.data.cursor='';this.data.hasMore=true;this.loadUnreadCount();this.loadNotifications(true).finally(()=>wx.stopPullDownRefresh())},
  onReachBottom(){if(this.data.hasMore&&!this.data.loading)this.loadNotifications()},
  loadUnreadCount(){api.getUnreadCount().then(d=>this.setData({unreadCount:d.count||0})).catch(()=>{})},
  loadNotifications(r){if(this.data.loading)return;this.setData({loading:true})
    api.listNotifications({cursor:r?'':this.data.cursor}).then(d=>{this.setData({notifications:r?(d.notifications||[]):this.data.notifications.concat(d.notifications||[]),cursor:d.next_cursor||'',hasMore:d.has_more,loading:false,unreadCount:d.unread_count||this.data.unreadCount})}).catch(()=>this.setData({loading:false}))},
  onReadAll(){api.markAllRead().then(()=>{const list=this.data.notifications.map(n=>({...n,is_read:true}));this.setData({notifications:list,unreadCount:0});wx.showToast({title:'已全部标记已读',icon:'success'})}).catch(()=>wx.showToast({title:'操作失败',icon:'none'}))},
  onTapNotify(e){const item=e.currentTarget.dataset.item;if(!item.is_read){api.markRead(item.id).then(()=>{item.is_read=true;this.setData({unreadCount:Math.max(0,this.data.unreadCount-1)})}).catch(()=>{})}
    if(item.ref_type==='post'&&item.ref_id)wx.navigateTo({url:'/pages/post/detail?id='+item.ref_id})},
  onDelete(e){const id=e.currentTarget.dataset.id;wx.showModal({title:'确认删除',content:'确定要删除这条通知吗？',success:r=>{if(r.confirm){api.deleteNotification(id).then(()=>{this.setData({notifications:this.data.notifications.filter(n=>n.id!==id)})}).catch(()=>wx.showToast({title:'删除失败',icon:'none'}))}}})}
})
