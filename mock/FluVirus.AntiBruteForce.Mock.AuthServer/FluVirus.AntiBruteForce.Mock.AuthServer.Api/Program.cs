using FluVirus.AntiBruteForce.Mock.AuthServer.Api.AntiBruteForce;
using FluVirus.AntiBruteForce.Mock.AuthServer.Api.ExceptionHandling;
using FluVirus.AntiBruteForce.Mock.AuthServer.Api.Resources;

namespace FluVirus.AntiBruteForce.Mock.AuthServer.Api;

public class Program
{
    public static void Main(string[] args)
    {
        WebApplicationBuilder builder = WebApplication.CreateBuilder(args);

        builder.Services.AddSingleton<ExceptionHandlerMiddleware>();
        builder.Services.AddSingleton<AntiBruteForceMiddleware>();

        AppConfiguration configuration = builder.Configuration.Get<AppConfiguration>() ?? throw new Exception("Cannot bind configuration");
        builder.Services.AddSingleton(configuration);

        WebApplication app = builder.Build();

        app.UseMiddleware<ExceptionHandlerMiddleware>();
        app.UseMiddleware<AntiBruteForceMiddleware>();

        app.MapGet("/resource", context =>
            context.Response.WriteAsJsonAsync(InMemoryResourceStorage.Default, context.RequestAborted)
        );

        app.MapGet("/echo/{value:int}", (HttpContext context, int value) =>
            context.Response.WriteAsJsonAsync(new Resource { Value = value }, context.RequestAborted)
        );

        app.Run();
    }
}
