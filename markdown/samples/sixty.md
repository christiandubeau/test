<script type="text/javascript" src="{{.Rootpath}}jwplayer/jwplayer.js"></script>

<div id="mediaplayer"></div>

<script type="text/javascript">
    jwplayer("mediaplayer").setup({
        "id": "playerID",
        "width": "480",
        "height": "270",
        "file": "{{.Rootpath}}/music/North of Sixty.mp3",
        "image": "",
        "modes": [
            {type: "html5"},
        ]
    });
</script>