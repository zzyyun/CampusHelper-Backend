const api=require('../../utils/api')
const {TASK_TYPE_TEXT}=require('../../utils/constants')
Page({
  data:{types:[1,2,3],taskType:1,title:'',description:'',location:'',rewardDesc:'',contact:'',note:'',expiredAt:'',submitting:false,typeText:TASK_TYPE_TEXT},
  onTypeChange(e){this.setData({taskType:parseInt(e.currentTarget.dataset.type)})},
  onInput(e){const f=e.currentTarget.dataset.field;if(f)this.setData({[f]:e.detail.value})},
  onSubmit(){if(!this.data.title.trim())return wx.showToast({title:'请输入任务标题',icon:'none'})
  if(!this.data.contact.trim())return wx.showToast({title:'请填写联系方式',icon:'none'})
    this.setData({submitting:true})
    api.createTask({task_type:this.data.taskType,title:this.data.title.trim(),description:this.data.description.trim(),location:this.data.location.trim(),reward_desc:this.data.rewardDesc.trim(),contact:this.data.contact.trim(),note:this.data.note.trim(),expired_at:this.data.expiredAt?parseInt(this.data.expiredAt):0}).then(()=>{this.setData({submitting:false});wx.showToast({title:'发布成功',icon:'success'});wx.switchTab({url:'/pages/tasks/tasks'})}).catch(()=>{this.setData({submitting:false});wx.showToast({title:'发布失败',icon:'none'})})}
})