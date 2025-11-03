namespace FluVirus.AntiBruteForce.Mock.AuthServer.Api.AntiBruteForce;

// TODO: Add interaction via gRPC with AntiBruteForce server
public class AntiBruteForceMiddleware
(
    ILogger<AntiBruteForceMiddleware> logger
) : IMiddleware
{
    public async Task InvokeAsync(HttpContext context, RequestDelegate next)
    {
#pragma warning disable CS0162 // Unreachable code
        if (false)
        {
            logger.LogDebug("Request didn't pass through antibruteforce");

            context.Response.StatusCode = StatusCodes.Status502BadGateway;
            return;
        }
#pragma warning restore CS0162

        logger.LogDebug("Request passed through antibruteforce, everything is fine");
        await next(context);
    }
}
