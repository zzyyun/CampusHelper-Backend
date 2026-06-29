const api=require('../../utils/api')
const {POST_TYPE_TEXT,ITEM_CATEGORY_TEXT,CONDITION_TEXT,TRADE_METHOD_TEXT,LOST_OR_FOUND_TEXT}=require('../../utils/constants')
Page({
  data:{type:1,title:'',content:'',images:[],lostOrFound:1,lostLocation:'',lostCategory:0,contact:'',
    price:'',originalPrice:'',condition:0,tradeMethod:1,sCategory:0,submitting:false,typeText:POST_TYPE_TEXT,
    lfItems:Object.entries(LOST_OR_FOUND_TEXT).map(([v,t])=>({value:parseInt(v),text:t})),
    condItems:Object.entries(CONDITION_TEXT).map(([v,t])=>({value:parseInt(v),text:t})),
    tradeItems:Object.entries(TRADE_METHOD_TEXT).map(([v,t])=>({value:parseInt(v),text:t})),
    lfText:'请选择',condText:'成色',tradeText:'交易方式'},
  onLoad(o){if(o.type)this.setData({type:parseInt(o.type)});this.updatePickText()},
  onTypeChange(e){this.setData({type:parseInt(e.currentTarget.dataset.type)})},
  onInput(e){const f=e.currentTarget.dataset.field;if(f)this.setData({[f]:e.detail.value})},
  onPick(e){const f=e.currentTarget.dataset.field;const arr=e.currentTarget.dataset.items;if(f&&arr){const items=this.data[arr]||[];const idx=e.detail.value;if(items[idx]){this.setData({[f]:items[idx].value});this.updatePickText()}}},
  updatePickText(){this.setData({lfText:LOST_OR_FOUND_TEXT[this.data.lostOrFound]||'请选择',condText:CONDITION_TEXT[this.data.condition]||'成色',tradeText:TRADE_METHOD_TEXT[this.data.tradeMethod]||'交易方式'})},
  chooseImage(){wx.chooseImage({count:9,sizeType:['compressed'],success:r=>{const tasks=r.tempFilePaths.map(p=>api.uploadFile(p,'post'));wx.showLoading({title:'上传中...',mask:true});Promise.all(tasks).then(results=>{wx.hideLoading();this.setData({images:results.map(r=>r.url).filter(Boolean)})}).catch(()=>wx.hideLoading())}})},
  removeImage(e){const im=[...this.data.images];im.splice(e.currentTarget.dataset.index,1);this.setData({images:im})},
  onSubmit(){if(!this.data.title.trim())return wx.showToast({title:'请输入标题',icon:'none'});if(!this.data.content.trim())return wx.showToast({title:'请输入内容',icon:'none'})
    this.setData({submitting:true});const data={type:this.data.type,title:this.data.title.trim(),content:this.data.content.trim(),images:this.data.images}
    if(this.data.type===2)data.lost_found={lost_or_found:this.data.lostOrFound,location:this.data.lostLocation,category:this.data.lostCategory||8,contact:this.data.contact}
    else if(this.data.type===3)data.second_hand={price:parseFloat(this.data.price)||0,original_price:parseFloat(this.data.originalPrice)||0,condition:this.data.condition||3,trade_method:this.data.tradeMethod||1,category:this.data.sCategory||8}
    api.createPost(data).then(()=>{this.setData({submitting:false});wx.showToast({title:'发布成功',icon:'success'});wx.switchTab({url:'/pages/home/home'})}).catch(()=>{this.setData({submitting:false});wx.showToast({title:'发布失败',icon:'none'})})}
})
