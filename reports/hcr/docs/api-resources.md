---
title: Api Resources
---

<table style="border-width:0; width:100%;">
{% for resource in config_sample.api_resources['api-resources'] %}
<tr><td id="td_table" colspan="2" style="border-width:5px">
  <b>Kind:</b> {{resource.kind}}
</td></tr>
<tr><td id="td_table" style="border-width:5px">{{resource.name}}</td>
<td id="td_table" style="border-width:5px">{{resource.groupVersion}}</td></tr>
<tr><td id="td_table" rowspan="3">
<b>Verbs:</b>
<ul>
{% for verb in resource.verbs %}<li>{{verb}}</li>{% endfor %}
</ul>
</td><td id="td_table"><b>Namespaced:</b> {{resource.namespaced}}</td></tr>
<tr><td id="td_table"><b>Short Names:</b>
<ul>
{% for sn in resource.shortNames %}<li>{{sn}}</li>{% endfor %}
</ul>
</td></tr>
<tr><td id="td_table"><b>Categories:</b>
<ul>
{% for cat in resource.categories %}<li>{{cat}}</li>{% endfor %}
</ul>
</td></tr>
<tr></tr>
<tr><td id="td_table" colspan="2"  style="border-width:1px"><b>filename:</b> {{resource.fileName}}</td></tr>
<tr><td id="td_footer" colspan="2"  style="border-width:1px; background-color:"><br/></td></tr>
{% endfor %}
</table>

