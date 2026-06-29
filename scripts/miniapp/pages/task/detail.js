const api=require('../../utils/api')
const {TASK_TYPE_TEXT,TASK_STATUS_TEXT,TASK_TYPE_TAG}=require('../../utils/constants')
Page({
  data:{task:null,claimContact:'',claimMessage:'',showClaimForm:false,
    taskTypeText:TASK_TYPE_TEXT,taskStatusText:TASK_STATUS_TEXT,taskTypeTag:TASK_TYPE_TAG},
  onLoad(o){if(o.id){this.data.taskId=o.id;this.loadTask()}},
  loadTask(){api.getTask(this.data.taskId).then(d=>{this.setData({task:d});wx.setNavigationBarTitle({title:d.title||'任务详情'})}).catch(()=>wx.showToast({title:'加载失败',icon:'none'}))},
  onClaim(){this.setData({showClaimForm:true})},
  cancelClaim(){this.setData({showClaimForm:false})},
  onInput(e){const f=e.currentTarget.dataset.field;if(f)this.setData({[f]:e.detail.value})},
  onSubmitClaim(){if(!this.data.claimContact.trim())return wx.showToast({title:'请填写联系方式',icon:'none'})
    wx.showLoading({title:'提交中...',mask:true})
    api.claimTask(this.data.taskId,{contact:this.data.claimContact.trim(),message:this.data.claimMessage.trim()}).then(d=>{wx.hideLoading();if(d.message){wx.showToast({title:'接单成功',icon:'success'});this.setData({showClaimForm:false});this.loadTask()}}).catch(()=>{wx.hideLoading();wx.showToast({title:'接单失败',icon:'none'})})},
  onComplete(){wx.showModal({title:'确认完成',content:'确定标记任务为已完成吗？',success:r=>{if(r.confirm){api.completeTask(this.data.taskId).then(()=>{wx.showToast({title:'已完成',icon:'success'});this.loadTask()}).catch(()=>wx.showToast({title:'操作失败',icon:'none'}))}}})},
  onCancel(){wx.showModal({title:'确认取消',content:'确定要取消此任务吗？',success:r=>{if(r.confirm){api.cancelTask(this.data.taskId).then(()=>{wx.showToast({title:'已取消',icon:'success'});this.loadTask()}).catch(()=>wx.showToast({title:'操作失败',icon:'none'}))}}})},
  onDelete(){wx.showModal({title:'确认删除',content:'确定要删除此任务吗？',success:r=>{if(r.confirm){api.deleteTask(this.data.taskId).then(()=>{wx.showToast({title:'已删除',icon:'success'});wx.navigateBack()}).catch(()=>wx.showToast({title:'删除失败',icon:'none'}))}}})}
})
