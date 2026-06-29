const api=require('../../utils/api')
Page({
  data:{items:[],cursor:'',hasMore:true,loading:false},
  onLoad(){this.loadAuditList()},
  loadAuditList(r){if(this.data.loading)return;this.setData({loading:true})
    api.adminListAudit({cursor:r?'':this.data.cursor}).then(d=>{this.setData({items:r?(d.items||[]):this.data.items.concat(d.items||[]),cursor:d.next_cursor||'',hasMore:d.has_more,loading:false})}).catch(()=>this.setData({loading:false}))},
  onApprove(e){const id=e.currentTarget.dataset.id;wx.showModal({title:'审核通过',content:'确定通过此内容吗？',success:r=>{if(r.confirm){wx.showLoading({title:'处理中...',mask:true});api.adminAuditContent(id,'approve','').then(()=>{wx.hideLoading();wx.showToast({title:'已通过',icon:'success'});this.setData({items:this.data.items.filter(i=>i.content_id!==id)})}).catch(()=>{wx.hideLoading();wx.showToast({title:'操作失败',icon:'none'})})}}})},
  onReject(e){const id=e.currentTarget.dataset.id;wx.showModal({title:'拒绝',content:'确定驳回此内容吗？',success:r=>{if(r.confirm){wx.showLoading({title:'处理中...',mask:true});api.adminAuditContent(id,'reject','不符合社区规范').then(()=>{wx.hideLoading();wx.showToast({title:'已驳回',icon:'success'});this.setData({items:this.data.items.filter(i=>i.content_id!==id)})}).catch(()=>{wx.hideLoading();wx.showToast({title:'操作失败',icon:'none'})})}}})},
  onScrollToLower(){if(this.data.hasMore&&!this.data.loading)this.loadAuditList()}
})
