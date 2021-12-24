## Most active GitHub users

This is a list of most active GitHub users in different countries/regions.
<ul class="country-list">
{% assign locations = (site.data.locations | sort) %}
{% for loc_hash in locations %}
  {% assign location = loc_hash[1] %}
  <li><a href="{{location.page | remove: '.html'}}">{{location.title}}</a></li>
{% endfor %}
</ul>
