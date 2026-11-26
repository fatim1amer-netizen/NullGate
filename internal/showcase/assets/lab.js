const $ = id => document.getElementById(id);
const APP_BASE = location.pathname.startsWith('/cyberlab/') ? '/cyberlab' : '';
function status(id, text, isError=false){const el=$(id);el.textContent=text;el.className='status '+(isError?'error':'ok')}
async function request(path, options={}) {
  const headers={'Accept':'application/json',...(options.headers||{})};
  if(options.body){headers['Content-Type']='application/json';const csrf=document.cookie.split('; ').find(x=>x.startsWith('csrf_token='));if(csrf)headers['X-CSRF-Token']=decodeURIComponent(csrf.slice(11))}
  const response=await fetch(APP_BASE+path,{credentials:'same-origin',...options,headers});
  const text=await response.text();let data;try{data=JSON.parse(text)}catch{data=text}
  return {response,data,text};
}
function showItems(id,items,render){const root=$(id);root.replaceChildren();for(const item of items){const node=document.createElement('div');node.className='item';render(node,item);root.append(node)}}
async function search(query=$('query').value){
  const q=encodeURIComponent(query);const result=await request('/api/products?q='+q);
  $('searchStatus').textContent='GET /api/products?q='+q+' — HTTP '+result.response.status;
  $('searchStatus').className='status '+(result.response.ok?'ok':'error');
  if(Array.isArray(result.data)){ $('productId').replaceChildren(); showItems('products',result.data,(node,item)=>{const title=document.createElement('strong');title.textContent=item.name+' · $'+(item.price_cents/100).toFixed(2);const desc=document.createElement('span');desc.textContent=item.description;const tag=document.createElement('div');tag.className='tag';tag.textContent=item.category;node.append(title,desc,tag);const option=document.createElement('option');option.value=item.id;option.textContent=item.name;$('productId').append(option)})}
  else $('products').textContent=result.data.error||'Unexpected response';
  $('rawResponse').textContent='GET /api/products?q='+q+'\nHTTP '+result.response.status+' '+result.response.statusText+'\n\n'+result.text;
}
async function refreshAccount(){
  const result=await request('/api/me');const authenticated=result.response.ok;
  $('logoutButton').hidden=!authenticated;$('bookingForm').querySelector('button').disabled=!authenticated;$('reviewForm').querySelector('button').disabled=!authenticated;
  if(authenticated){status('authStatus','Signed in as '+result.data.name+' ('+result.data.email+')');refreshBookings()}
  else status('authStatus','Not signed in. Create a local account to book or submit reviews.',true);
}
async function auth(path){
  const payload={Email:$('email').value,Password:$('password').value};
  if(path==='/api/register')payload.Name=$('name').value;
  const result=await request(path,{method:'POST',body:JSON.stringify(payload)});
  status('authStatus',result.data.error||('Welcome, '+(result.data.name||'')),!result.response.ok);
  if(result.response.ok)refreshAccount();
}
async function refreshBookings(){
  const result=await request('/api/bookings');if(!result.response.ok)return;
  showItems('bookings',result.data,(node,item)=>{const title=document.createElement('strong');title.textContent=item.destination+' · '+item.travel_date;const detail=document.createElement('span');detail.textContent=item.guests+' guest(s) · Booking #'+item.id;nodeAppend(node,title,detail)});
}
function nodeAppend(item,...nodes){nodes.forEach(node=>item.append(node))}
$('searchForm').addEventListener('submit',event=>{event.preventDefault();search()});
$('loginButton').addEventListener('click',()=>auth('/api/login'));
$('registerButton').addEventListener('click',()=>auth('/api/register'));
$('logoutButton').addEventListener('click',async()=>{await request('/api/logout',{method:'POST',body:'{}'});refreshAccount()});
$('bookingForm').addEventListener('submit',async event=>{event.preventDefault();const result=await request('/api/bookings',{method:'POST',body:JSON.stringify({Destination:$('destination').value,TravelDate:$('travelDate').value,Guests:Number($('guests').value)})});status('bookingStatus',result.data.error||('Booking saved: #'+result.data.id),!result.response.ok);if(result.response.ok)refreshBookings()});
$('reviewForm').addEventListener('submit',async event=>{event.preventDefault();const result=await request('/api/reviews',{method:'POST',body:JSON.stringify({ProductID:Number($('productId').value),Rating:Number($('rating').value),Body:$('reviewText').value})});status('reviewStatus',result.data.error||'Review saved.',!result.response.ok)});
document.querySelectorAll('.payload').forEach(button=>button.addEventListener('click',()=>{$('rawQuery').value=button.dataset.value;$('query').value=button.dataset.value;search(button.dataset.value)}));
$('rawQuery').addEventListener('change',()=>{$('query').value=$('rawQuery').value});
$('rawQuery').addEventListener('input',()=>{$('query').value=$('rawQuery').value});
$('activityButton').addEventListener('click',async()=>{const result=await request('/api/lab/events');if(!result.response.ok){$('activity').textContent=result.data.error;return}showItems('activity',result.data,(node,item)=>{const title=document.createElement('strong');title.textContent=item.action;const detail=document.createElement('span');detail.textContent=item.details+' · '+item.created_at;nodeAppend(node,title,detail)})});
search();refreshAccount();
