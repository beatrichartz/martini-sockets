var ChatRoom = function() {
  var c = this;
  
  c.create = function() {
    c.room       = document.getElementById("room");
    
    c.nameForm   = document.getElementById("name-form");
    c.nameInput  = c.nameForm.querySelector("#name-input");
    
    c.sendForm   = document.getElementById("send-form");
    c.textInput  = c.sendForm.querySelector("#text-input");
    
    return c;
  };
  
  c.connect = function() {
    c.ws = new WebSocket("ws://" + location.host + "/sockets" + location.pathname + '/' + c.client.name);
    return c;
  };
  
  c.disconnect = function() {
    c.ws.close();
  };
  
  c.listen = function() {
    c.ws.addEventListener("message", function(e) {
      c.write(JSON.parse(e.data), 'right');
    });
    
    c.ws.addEventListener("close", function(e) {
      console.log("closed connection, reconnecting", e)
      c.connect()
    });
    
    c.ws.addEventListener("error", function(e) {
      console.log("error for connection", e)
    });
    
    return c;
  };
  
  c.send = function(messageData) {
    c.ws.send(JSON.stringify(messageData));
  };
  
  c.handleSend = function() {
    c.sendForm.addEventListener("submit", function(e) {
      e.preventDefault();
      
      if (c.textInput.value && c.textInput.value.length) {
        var messageData = {
          typ: "message",
          text: c.textInput.value
        };
        
        c.send(messageData);
        
        messageData.from = c.client.name;
        c.write(messageData, 'left');
        
        c.textInput.value = ''
      }
      
      return false;
    });
    
    return c;
  };
  
  c.handleName = function() {
    c.client = { name: "Anonymous" }
    c.nameForm.addEventListener("submit", function(e) {
      e.preventDefault();
      
      if (c.nameInput.value && c.nameInput.value.length) {
        c.client.name = c.nameInput.value;
      }
      
      c.connect().listen();
      
      c.nameForm.parentElement.removeChild(c.nameForm);
      c.sendForm.parentElement.removeAttribute("class");
      
      return false;
    });
    
    return c;
  }
  
  c.write = function(messageData, className) {
    var container  = document.createElement("p"),
        name       = document.createElement("span"),
        message    = document.createElement("span");
    
    name.innerHTML    = messageData.from;
    message.innerHTML = messageData.text;
    container.appendChild(name);
    container.appendChild(message);
    c.room.appendChild(container);
    
    requestAnimationFrame(function() {
      container.className += className + ' show ' + messageData.typ;
    });
    
    window.scrollTo(0, document.documentElement.scrollTop + 1000)
  };


  var initialize = function() {
    c.create().handleName().handleSend();
  }();
};

var RoomForm = function() {
  var r = this;
  
  r.create = function() {
    r.form      = document.getElementById("room-form");
    r.roomInput = r.form.querySelector("#room-input");
    
    return r;
  };
  
  r.handleForm = function() {
    r.form.addEventListener("submit", function(e) {
      e.preventDefault();
      
      if (r.roomInput.value && r.roomInput.value.length) {
        location.replace("/rooms/" + r.roomInput.value);
      }
      
      return false;
    });
    
    return r;
  }
  
  var initialize = function() {
    r.create().handleForm().handleAway();
  }();
}

var chat;

window.addEventListener("load", function() {
  if (location.pathname != '/') {
    chat = new ChatRoom();
  } else {
    new RoomForm();
  }
});