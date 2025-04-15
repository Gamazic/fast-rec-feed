math.randomseed(os.time())

function request()
    local id = math.random(0, 5000000)
    return wrk.format("GET", "/feed/" .. id)
end 
