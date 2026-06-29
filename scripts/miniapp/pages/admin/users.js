const api=require('../../utils/api')
Page({
  data:{users:[],keyword:'',cursor:'',hasMore:true,loading:false,showBanModal:false,banUserId:0,banReason:''},
  onLoad(){this.loadUsers()},
  onInput(e){const f=e.currentTarget.dataset.field;if(f)this.setData({[f]:e.detail.value})},
  onSearch(){this.data.cursor='';this.loadUsers(true)},
  loadUsers(r){if(this.data.loading)return;this.setData({loading:true})
    api.adminListUsers({keyword:this.data.keyword,cursor:r?'':this.data.cursor}).then(d=>{this.setData({users:r?(d.users||[]):this.data.users.concat(d.users||[]),cursor:d.next_cursor||'',hasMore:d.has_more,loading:false})}).catch(()=>this.setData({loading:false}))},
  onBan(e){this.setData({showBanModal:true,banUserId:e.currentTarget.dataset.id,banReason:''})},
  cancelBan(){this.setData({showBanModal:false})},
  submitBan(){if(!this.data.banReason.trim())return wx.showToast({title:'请填写封禁原因',icon:'none'})
    wx.showLoading({title:'处理中...',mask:true});api.adminBanUser(this.data.banUserId,this.data.banReason).then(()=>{wx.hideLoading();wx.showToast({title:'已封禁',icon:'success'});this.setData({showBanModal:false});this.loadUsers(true)}).catch(()=>{wx.hideLoading();wx.showToast({title:'操作失败',icon:'none'})})},
  onUnban(e){const id=e.currentTarget.dataset.id;wx.showModal({title:'确认解封',content:'确定要解封该用户吗？',success:r=>{if(r.confirm){api.adminUnbanUser(id).then(()=>{wx.showToast({title:'已解封',icon:'success'});this.loadUsers(true)}).catch(()=>wx.showToast({title:'操作失败',icon:'none'}))}}})},
  onScrollToLower(){if(this.data.hasMore&&!this.data.loading)this.loadUsers()}
})
